// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"gopmgr/internal/auth"
	"gopmgr/internal/crypto"
)

// Sign-in bound to one account incarnation (the credential-coherence slice
// of docs/design/wbs-authenticated-account-identity.md). A username can be
// purged and created again, so checking a password and then unlocking,
// stamping, or enrolling "username" in separate steps could act on a
// different account than the one whose password was checked.
//
// Each attempt reads one snapshot (identity, hash, key wrap), does the slow
// password and key work outside any lock, then, in one write transaction,
// requires the same identity and the same hash and wrap before writing
// anything. A different identity is refused like an unknown user; a changed
// hash or wrap of the same identity is retried from the start, so the
// password is checked against what is stored now.

// ErrSignInContended is returned when the account's password or key kept
// changing while a sign-in was checking it.
var ErrSignInContended = errors.New("users: the account changed during sign-in")

const signInAttempts = 3

// beforeCredentialCommit runs on every attempt after the password and key
// work and before the write transaction begins. Tests replace it to change
// the account in between, as another process could.
var beforeCredentialCommit = func() {}

// errStaleCredentials ends an attempt whose snapshot no longer matches.
var errStaleCredentials = errors.New("users: credentials changed during sign-in")

// Principal is the account incarnation a sign-in verified: its immutable
// identity, its username, and its authorization generation after the
// sign-in's own writes. It is deliberately not part of Account, which is
// sent to the frontend.
type Principal struct {
	id         string
	username   string
	generation int64
}

// ID returns the immutable account identity.
func (p Principal) ID() string { return p.id }

// Username returns the account's username.
func (p Principal) Username() string { return p.username }

// Generation returns the authorization generation the sign-in established.
func (p Principal) Generation() int64 { return p.generation }

// credentialRow is one read of an account and its identity.
type credentialRow struct {
	account    Account
	id         string
	generation int64
	hash       string
	wrapped    string
}

func (s *Store) readCredentialRow(ctx context.Context, q accountWriter, username string) (credentialRow, error) {
	var (
		r                    credentialRow
		storedDataDir        string
		createdAt, lastLogin string
		isAdmin, disabled    int
		id                   sql.NullString
		generation           sql.NullInt64
	)
	err := q.QueryRowContext(ctx,
		`SELECT u.username, u.display_name, u.password_hash, u.wrapped_dek_pw, u.data_dir, u.created_at, u.last_login,
		        u.is_admin, u.last_export_directory, u.disabled, i.id, i.generation
		 FROM users u LEFT JOIN account_identity_registry i ON i.username = u.username
		 WHERE u.username = ?`, username,
	).Scan(&r.account.Username, &r.account.DisplayName, &r.hash, &r.wrapped, &storedDataDir, &createdAt, &lastLogin,
		&isAdmin, &r.account.LastExportDirectory, &disabled, &id, &generation)
	if errors.Is(err, sql.ErrNoRows) {
		return credentialRow{}, ErrNoSuchUser
	}
	if err != nil {
		return credentialRow{}, err
	}
	r.id, r.generation = id.String, generation.Int64
	// data_dir is recomputed, not trusted: see List.
	r.account.DataDir = filepath.Join(s.rootDir, r.account.Username)
	r.account.IsAdmin = isAdmin == 1
	r.account.Disabled = disabled == 1
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		r.account.CreatedAt = t
	}
	if lastLogin != "" {
		if t, err := time.Parse(time.RFC3339Nano, lastLogin); err == nil {
			r.account.LastLogin = t
		}
	}
	return r, nil
}

// credentialWork is an attempt's password and key work, done before its
// write transaction: the unlocked key, and the new hash and key wrap to
// store, if any.
type credentialWork struct {
	dek     []byte
	newHash string // non-empty: rehash with stronger parameters
	newWrap string // non-empty: the account had no key yet, and this is it
}

// checkPassword verifies password against snapshot. An account with no
// identity is refused only after that, so the refusal tells nothing to
// someone who does not know the password.
func checkPassword(snapshot credentialRow, password string) error {
	if err := auth.VerifyPassword(password, snapshot.hash); err != nil {
		return err
	}
	if snapshot.id == "" {
		return ErrAccountIdentity
	}
	return nil
}

// unlockKey unwraps snapshot's DEK with the verified password or, for an
// account that predates ADR-001, makes one and wraps it.
func unlockKey(snapshot credentialRow, password string) (credentialWork, error) {
	if snapshot.wrapped != "" {
		dek, err := crypto.UnwrapKey(snapshot.wrapped, password)
		if err != nil {
			return credentialWork{}, err
		}
		return credentialWork{dek: dek}, nil
	}
	dek, err := crypto.GenerateDEK()
	if err != nil {
		return credentialWork{}, err
	}
	wrap, err := crypto.WrapKey(dek, password)
	if err != nil {
		clear(dek)
		return credentialWork{}, err
	}
	return credentialWork{dek: dek, newWrap: wrap}, nil
}

// prepareSignIn is a sign-in attempt's work before its transaction.
// ErrNoSuchUser and auth.ErrMismatch come before any other error.
func prepareSignIn(snapshot credentialRow, password string) (credentialWork, error) {
	if err := checkPassword(snapshot, password); err != nil {
		return credentialWork{}, err
	}
	// Checked only after the password, so it tells nothing to someone who
	// does not know it.
	if snapshot.account.Disabled {
		return credentialWork{}, ErrAccountDisabled
	}
	var newHash string
	if auth.NeedsRehash(snapshot.hash) {
		h, err := auth.HashPassword(password)
		if err != nil {
			return credentialWork{}, fmt.Errorf("users: rehash password: %w", err)
		}
		newHash = h
	}
	w, err := unlockKey(snapshot, password)
	if err != nil {
		return credentialWork{}, err
	}
	w.newHash = newHash
	return w, nil
}

// requireSameCredentialsTx re-reads username in the caller's transaction
// and checks it is the incarnation, hash, and key wrap of snapshot.
func (s *Store) requireSameCredentialsTx(ctx context.Context, q accountWriter, snapshot credentialRow) (credentialRow, error) {
	now, err := s.readCredentialRow(ctx, q, snapshot.account.Username)
	if err != nil {
		return credentialRow{}, err
	}
	if now.id != snapshot.id {
		// The account whose password was checked is gone; this is another
		// one with the same name.
		return credentialRow{}, ErrNoSuchUser
	}
	if now.hash != snapshot.hash || now.wrapped != snapshot.wrapped {
		return credentialRow{}, errStaleCredentials
	}
	if now.account.Disabled {
		return credentialRow{}, ErrAccountDisabled
	}
	return now, nil
}

// storeCredentialWorkTx writes an attempt's new hash and key wrap, each
// only over the value the attempt read.
func storeCredentialWorkTx(ctx context.Context, q accountWriter, username string, snapshot credentialRow, w credentialWork) error {
	if w.newHash != "" {
		res, err := q.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE username = ? AND password_hash = ?`,
			w.newHash, username, snapshot.hash)
		if err := oneRowChanged(res, err, "persist password rehash"); err != nil {
			return err
		}
	}
	if w.newWrap != "" {
		res, err := q.ExecContext(ctx, `UPDATE users SET wrapped_dek_pw = ? WHERE username = ? AND wrapped_dek_pw = ''`,
			w.newWrap, username)
		if err := oneRowChanged(res, err, "store encryption key"); err != nil {
			return err
		}
	}
	return nil
}

func oneRowChanged(res sql.Result, err error, op string) error {
	if err != nil {
		return fmt.Errorf("users: %s: %w", op, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("users: %s: %w", op, err)
	}
	if n != 1 {
		return fmt.Errorf("users: %s: %w", op, errStaleCredentials)
	}
	return nil
}

// generationTx reads username's authorization generation.
func generationTx(ctx context.Context, q accountWriter, username string) (int64, error) {
	var g int64
	err := q.QueryRowContext(ctx, `SELECT generation FROM account_identity_registry WHERE username = ?`, username).Scan(&g)
	return g, err
}

// enrollInSignInSavepoint runs enroll in a savepoint of the sign-in
// transaction: a failure rolls back only the enrollment, is logged, and
// is retried at the next sign-in, as before.
func enrollInSignInSavepoint(ctx context.Context, q accountWriter, username string, enroll func() error) error {
	if _, err := q.ExecContext(ctx, `SAVEPOINT sign_in_escrow`); err != nil {
		return err
	}
	enrollErr := enroll()
	if enrollErr != nil {
		if _, err := q.ExecContext(ctx, `ROLLBACK TO sign_in_escrow`); err != nil {
			return errors.Join(enrollErr, err)
		}
		log.Printf("users: escrow enrollment of %s failed; retried at next sign-in: %v", username, enrollErr)
	}
	_, err := q.ExecContext(ctx, `RELEASE sign_in_escrow`)
	return err
}

// signIn is SignIn and SignInHeadless. adminSession runs the
// administrator's escrow session (EnrollAdminSession) in the sign-in
// transaction; without it, only a key made by this sign-in is enrolled.
func (s *Store) signIn(username, password string, adminSession bool) (Account, Principal, []byte, error) {
	if err := ValidateUsername(username); err != nil {
		return Account{}, Principal{}, nil, ErrNoSuchUser
	}
	ctx := context.Background()
	for attempt := 0; attempt < signInAttempts; attempt++ {
		snapshot, err := s.readCredentialRow(ctx, s.conn, username)
		if err != nil {
			return Account{}, Principal{}, nil, err
		}
		work, err := prepareSignIn(snapshot, password)
		if err != nil {
			return Account{}, Principal{}, nil, err
		}
		beforeCredentialCommit()

		var acc Account
		var principal Principal
		err = s.inWriteTx("sign-in", func(ctx context.Context, q accountWriter) error {
			now, err := s.requireSameCredentialsTx(ctx, q, snapshot)
			if err != nil {
				return err
			}
			acc = now.account
			stamp := time.Now().UTC()
			if _, err := q.ExecContext(ctx, `UPDATE users SET last_login = ? WHERE username = ?`,
				stamp.Format(time.RFC3339Nano), username); err != nil {
				return fmt.Errorf("users: update last_login: %w", err)
			}
			if err := storeCredentialWorkTx(ctx, q, username, snapshot, work); err != nil {
				return err
			}
			if adminSession {
				err = enrollInSignInSavepoint(ctx, q, username, func() error {
					return enrollAdminSessionTx(ctx, q, username, work.dek)
				})
			} else if work.newWrap != "" {
				err = enrollInSignInSavepoint(ctx, q, username, func() error {
					_, err := enrollTx(ctx, q, username, work.dek, nil)
					return err
				})
			}
			if err != nil {
				return err
			}
			generation, err := generationTx(ctx, q, username)
			if err != nil {
				return fmt.Errorf("users: read account generation: %w", err)
			}
			principal = Principal{id: now.id, username: username, generation: generation}
			return nil
		})
		if errors.Is(err, errStaleCredentials) {
			clear(work.dek)
			continue
		}
		if err != nil {
			clear(work.dek)
			return Account{}, Principal{}, nil, err
		}
		// The returned Account is the one read before the stamp, so its
		// LastLogin is the previous sign-in.
		return acc, principal, work.dek, nil
	}
	return Account{}, Principal{}, nil, ErrSignInContended
}

// SignIn verifies username's password, unlocks its DEK, records the
// sign-in, and runs the administrator's escrow session, all for the one
// account incarnation whose password was checked. It returns that
// account, its Principal, and the DEK, which the caller must clear.
//
// Unknown users and wrong passwords return ErrNoSuchUser or
// auth.ErrMismatch, which callers show alike; ErrAccountDisabled and
// ErrAccountIdentity come only after the password matched. An escrow
// failure does not stop the sign-in; it is logged and retried next time.
func (s *Store) SignIn(username, password string) (Account, Principal, []byte, error) {
	return s.signIn(username, password, true)
}

// SignInHeadless is SignIn for command-line maintenance: it records the
// sign-in and unlocks the DEK, but runs no administrator escrow session.
// A key it creates for an account that had none is still enrolled.
func (s *Store) SignInHeadless(username, password string) (Account, Principal, []byte, error) {
	return s.signIn(username, password, false)
}

// IssueRecoveryCodesWithPassword issues username's recovery codes on behalf
// of actor, an enabled administrator, with the account's password, which
// must unlock its DEK (an account without one gets it now, enrolled as at
// sign-in). Only the super administrator may issue an administrator's
// codes (ErrNotSuper). The roles, the identity, and the key are checked in
// the transaction that stores the codes, so codes are never stored for a
// different account than the one whose key they wrap.
func (s *Store) IssueRecoveryCodesWithPassword(actor, username, password string) ([]string, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, ErrNoSuchUser
	}
	ctx := context.Background()
	for attempt := 0; attempt < signInAttempts; attempt++ {
		snapshot, err := s.readCredentialRow(ctx, s.conn, username)
		if err != nil {
			return nil, err
		}
		// Neither the account being disabled nor a weak hash matters here,
		// as they did not before: the codes only wrap its key.
		if err := checkPassword(snapshot, password); err != nil {
			return nil, err
		}
		work, err := unlockKey(snapshot, password)
		if err != nil {
			if snapshot.wrapped != "" {
				return nil, fmt.Errorf("%w: %v", ErrPasswordWrapCorrupt, err)
			}
			return nil, err
		}
		set, err := newRecoveryCodeSet(work.dek)
		if err != nil {
			clear(work.dek)
			return nil, err
		}
		beforeCredentialCommit()

		err = s.inWriteTx("recovery code issue", func(ctx context.Context, q accountWriter) error {
			if err := requireEnabledAdmin(ctx, q, actor); err != nil {
				return err
			}
			now, err := s.readCredentialRow(ctx, q, username)
			if err != nil {
				return err
			}
			if now.id != snapshot.id {
				return ErrNoSuchUser
			}
			if now.hash != snapshot.hash || now.wrapped != snapshot.wrapped {
				return errStaleCredentials
			}
			if now.account.IsAdmin {
				if err := requireSuperTx(ctx, q, actor); err != nil {
					return err
				}
			}
			if err := storeCredentialWorkTx(ctx, q, username, snapshot, work); err != nil {
				return err
			}
			if work.newWrap != "" {
				if err := enrollInSignInSavepoint(ctx, q, username, func() error {
					_, err := enrollTx(ctx, q, username, work.dek, nil)
					return err
				}); err != nil {
					return err
				}
			}
			return replaceRecoveryCodes(ctx, q, username, set)
		})
		clear(work.dek)
		if errors.Is(err, errStaleCredentials) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return set.plain, nil
	}
	return nil, ErrSignInContended
}
