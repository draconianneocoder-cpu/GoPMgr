#!/bin/bash
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Runs check-escrow-release-guard.sh against fixture trees: it must fail
# whenever non-test Go code seals a DEK, however it is written, and pass
# otherwise. Nothing may lift it short of deleting it.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CHECK="$ROOT/scripts/check-escrow-release-guard.sh"

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

run_fixture internal/users/x.go 'package users' ||
	fail "no sealing must pass"
run_fixture internal/crypto/escrow.go 'package crypto
func SealDEK() {}
func helper() { SealDEK() }' ||
	fail "SealDEK inside internal/crypto is the primitive, not sealing"
run_fixture internal/users/escrow_test.go 'package users
func seal() { crypto.SealDEK(); db.Exec("INSERT INTO sealed_deks") }' ||
	fail "sealing in a test file must not count"
run_fixture internal/users/escrow.go 'package users
// sealed_deks rows go with the account (ON DELETE CASCADE).
const schema = "CREATE TABLE IF NOT EXISTS sealed_deks (username TEXT)"' ||
	fail "creating or describing the table is not sealing"

expect_fail internal/users/escrow.go 'package users
func seal() { crypto.SealDEK(nil, "", "", nil) }'
expect_fail internal/users/escrow.go 'package users
import c "gopmgr/internal/crypto"
func seal() { c.SealDEK(nil, "", "", nil) }'
expect_fail internal/users/escrow.go 'package users
func store() { q.Exec(`INSERT INTO sealed_deks (username) VALUES (?)`) }'
expect_fail internal/users/escrow.go 'package users
func store() { q.Exec(`insert or replace into sealed_deks (username) values (?)`) }'
expect_fail internal/users/escrow.go 'package users
func store() { q.Exec(`UPDATE sealed_deks SET sealed = ?`) }'
expect_fail internal/users/escrow.go 'package users
func seal() { crypto.SealDEK(nil, "", "", nil) }' app_access.go 'package main
func access() { record(users.AccountAdminAccess) }'
echo "check-escrow-release-guard tests passed."
