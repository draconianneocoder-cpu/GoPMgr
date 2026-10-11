// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"bytes"
	"testing"

	"gopmgr/internal/crypto"
)

func TestReplacementRequiresAnOpenableActorGrant(t *testing.T) {
	for _, damage := range []string{"public key", "sealed DEK", "unopenable DEK"} {
		t.Run(damage, func(t *testing.T) {
			store, dek := escrowStore(t)
			old := openEscrow(t, store, "alice", dek)
			code := mustCreateRecoveryCode(t, store, "alice", dek)
			switch damage {
			case "public key":
				_, public := attackerKeyPair(t)
				mustExec(t, store, `UPDATE personal_keys SET public_key = ? WHERE username = 'alice'`, public)
			case "sealed DEK":
				sealed, err := crypto.SealDEK(old.public, old.id, "alice", bytes.Repeat([]byte{42}, 32))
				if err != nil {
					t.Fatal(err)
				}
				mustExec(t, store, `UPDATE sealed_deks SET sealed = ? WHERE username = 'alice'`, sealed)
			case "unopenable DEK":
				mustExec(t, store, `UPDATE sealed_deks SET sealed = x'00' WHERE username = 'alice'`)
			}
			before := len(eventActions(t, store))
			if _, err := store.ReplaceEscrowKey("alice", dek, ""); err == nil {
				t.Fatal("replacement succeeded without an openable actor grant")
			}
			if activeKeyID(t, store) != old.id || countRows(t, store, "escrow_keys", "1 = 1") != 1 {
				t.Fatal("failed replacement changed the active key")
			}
			if countRows(t, store, "escrow_recovery", "1 = 1") != 1 || len(eventActions(t, store)) != before {
				t.Fatal("failed replacement changed recovery or account history")
			}
			mustExec(t, store, `INSERT INTO escrow_rotation_pending (id, reason, marked_at) VALUES (1, 'previous change', ?)`, nowStamp())
			if err := store.RunPendingRotation("alice", dek); err != nil {
				t.Fatalf("pending replacement should remain retryable: %v", err)
			}
			if activeKeyID(t, store) != old.id || !rotationPending(t, store) ||
				countRows(t, store, "escrow_recovery", "1 = 1") != 1 || len(eventActions(t, store)) != before {
				t.Fatal("automatic replacement did not roll back and remain pending")
			}
			openEscrow(t, store, "alice", dek)
			if err := store.ConfirmEscrowRecoveryCode("alice", code); err != nil {
				t.Fatalf("original recovery code no longer works: %v", err)
			}
		})
	}
}

func TestHandOverDoesNotScheduleWithoutAnActiveKeyOrOnRejection(t *testing.T) {
	t.Run("no active key", func(t *testing.T) {
		store, aliceDEK, _ := superStore(t)
		mustExec(t, store, `UPDATE escrow_keys SET retired_at = ?`, nowStamp())
		if passed, err := store.HandOverSuper("alice", aliceDEK, "bob"); err != nil || passed {
			t.Fatalf("hand-over without an active key = %v, %v", passed, err)
		}
		if rotationPending(t, store) || mustSuper(t, store) != "bob" {
			t.Fatal("hand-over created a pending replacement without an active key")
		}
	})
	t.Run("rejected target", func(t *testing.T) {
		store, aliceDEK, _ := superStore(t)
		before := len(eventActions(t, store))
		if _, err := store.HandOverSuper("alice", aliceDEK, "carol"); err == nil {
			t.Fatal("hand-over to a standard account succeeded")
		}
		if rotationPending(t, store) || mustSuper(t, store) != "alice" || len(eventActions(t, store)) != before {
			t.Fatal("rejected hand-over changed roles, history, or pending replacement")
		}
		openEscrow(t, store, "alice", aliceDEK)
	})
}

func TestHandOverSchedulesReplacementWithoutADepartingGrant(t *testing.T) {
	for _, damage := range []string{"unopenable grant", "absent grant", "already pending"} {
		t.Run(damage, func(t *testing.T) {
			store, aliceDEK, bobDEK := superStore(t)
			code := mustCreateRecoveryCode(t, store, "alice", aliceDEK)
			oldID := activeKeyID(t, store)
			if damage == "unopenable grant" {
				mustExec(t, store, `UPDATE escrow_grants SET sealed = x'00' WHERE admin_username = 'alice'`)
			} else {
				mustExec(t, store, `DELETE FROM escrow_grants WHERE admin_username = 'alice'`)
			}
			if damage == "already pending" {
				mustExec(t, store, `INSERT INTO escrow_rotation_pending (id, reason, marked_at) VALUES (1, 'previous change', ?)`, nowStamp())
			}
			if passed, err := store.HandOverSuper("alice", aliceDEK, "bob"); err != nil || passed {
				t.Fatalf("hand-over = %v, %v; want role moved without key", passed, err)
			}
			if !rotationPending(t, store) {
				t.Fatal("hand-over failed to schedule key replacement")
			}
			if err := store.RecoverEscrowKey("bob", bobDEK, code); err != nil {
				t.Fatalf("new super administrator recovery: %v", err)
			}
			if activeKeyID(t, store) == oldID || rotationPending(t, store) || countRows(t, store, "escrow_recovery", "1 = 1") != 0 {
				t.Fatal("recovery did not replace the departing administrator's key and code")
			}
			openEscrow(t, store, "bob", bobDEK)
		})
	}
}
