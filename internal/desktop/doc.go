// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Package desktop is the GoPMgr application layer: the Wails-bound App, its
// window, menu and lifecycle, and the command-line and headless project
// modes. The root main package only embeds the built frontend and calls Main.
//
// App is the single object Wails binds. Every exported App method is
// callable from the Svelte frontend as window.go.desktop.App.<Method>, so a
// new exported method is a new frontend API; helpers stay unexported or
// package-level. The methods are grouped by domain in app_*.go files but
// stay on one struct in one package, because Wails binds one object and the
// methods share its session, lock and open project. Domain logic lives in
// the other internal packages; App methods authorize, translate the wire
// format, and delegate.
//
// Accounts (Argon2id) live in a system database under users.DefaultRootDir;
// a pre-rename "PMForge" install is copied forward on first launch
// (users.MigrateLegacyRoot). CLI dispatch (--version, --check, --repair,
// ...) runs without the GUI.
package desktop
