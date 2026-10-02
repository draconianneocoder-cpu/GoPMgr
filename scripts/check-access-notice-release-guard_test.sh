#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Runs check-access-notice-release-guard.sh against fixture trees: it must
# fail whenever non-test code seals a DEK or, outside internal/users, opens
# another account's data, however it is written, and pass otherwise. Nothing
# may lift it short of deleting it.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/check-access-notice-release-guard.sh"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

# run_fixture <path> <body> [<path> <body>]... builds a tree, then runs the
# guard against it.
run_fixture() {
	local tmp result
	tmp="$(mktemp -d)"
	while [ "$#" -gt 1 ]; do
		mkdir -p "$tmp/$(dirname "$1")"
		printf '%s\n' "$2" >"$tmp/$1"
		shift 2
	done
	result=0
	GOPMGR_GUARD_ROOT="$tmp" bash "$CHECK" >/dev/null 2>&1 || result=$?
	rm -rf "$tmp"
	return "$result"
}

expect_fail() {
	if run_fixture "$@"; then
		fail "guard passed on: $*"
	fi
}

run_fixture main.go 'package main' ||
	fail "no access must pass"
run_fixture internal/users/escrow_access.go 'package users
func (s *Store) OpenUserForAdmin() {}
func helper(s *Store) { s.OpenUserForAdmin() }' ||
	fail "the definition in internal/users is not the app opening data"
run_fixture app_admin_access_test.go 'package main
func open(a *App) { a.store.OpenUserForAdmin("a", nil, "b", "r") }' ||
	fail "a call in a test file must not count"
run_fixture app_admin_access.go 'package main
// AdminOpenUserData calls users.Store.OpenUserForAdmin.
func x() {}' ||
	fail "naming the method in a comment is not calling it"

run_fixture internal/crypto/escrow.go 'package crypto
func SealDEK() {}
func helper() { SealDEK() }' ||
	fail "SealDEK inside internal/crypto is the primitive, not sealing"
run_fixture internal/users/escrow.go 'package users
// sealed_deks rows go with the account (ON DELETE CASCADE).
const schema = "CREATE TABLE IF NOT EXISTS sealed_deks (username TEXT)"' ||
	fail "creating or describing the table is not sealing"

expect_fail app_admin_access.go 'package main
func open(a *App) { a.store.OpenUserForAdmin("a", nil, "b", "r") }'
# Sealing alone, with no access call: phase 1's unrecorded-access risk.
expect_fail internal/users/escrow.go 'package users
func seal() { crypto.SealDEK(nil, "", "", nil) }'
expect_fail internal/users/escrow.go 'package users
import c "gopmgr/internal/crypto"
func seal() { c.SealDEK(nil, "", "", nil) }'
expect_fail internal/users/escrow.go 'package users
func store() { q.Exec(`insert or replace into sealed_deks (username) values (?)`) }'
expect_fail internal/users/escrow.go 'package users
func store() { q.Exec(`UPDATE sealed_deks SET sealed = ?`) }'
# Access renamed, or reached through a wrapper inside internal/users: the
# sealing it needs still fails the guard.
expect_fail internal/users/escrow.go 'package users
func seal() { crypto.SealDEK(nil, "", "", nil) }
func (s *Store) ViewUser() {}' app_admin_access.go 'package main
func open(a *App) { a.store.ViewUser() }'
expect_fail internal/users/escrow.go 'package users
func store() { q.Exec(`INSERT INTO sealed_deks (username) VALUES (?)`) }
func (s *Store) Wrap() { s.OpenUserForAdmin() }' app_admin_access.go 'package main
func open(a *App) { a.store.Wrap() }'
expect_fail internal/cli/access.go 'package cli
func open(st *users.Store) { _, _ = st.OpenUserForAdmin("a", nil, "b", "r") }'
expect_fail app_admin_access.go 'package main
func open(a *App) { a.store.OpenUserForAdmin("a", nil, "b", "r") }' app_notice.go 'package main
func notice() { record(users.AccountAdminAccess) }'
echo "check-access-notice-release-guard tests passed."
