// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Command gopmgr is the GoPMgr desktop application. It embeds the built
// frontend and hands control to internal/desktop. The main package stays at
// the repository root because `wails build` requires it here, and go:embed
// cannot reach frontend/dist from a subdirectory.
package main

import (
	"embed"

	"gopmgr/internal/desktop"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	desktop.Main(assets)
}
