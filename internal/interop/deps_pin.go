// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build interop_deps_pin

// Package interop is the root of the ADR-010 worker interop packages.
//
// This file only pins the OWN-11 dependencies approved for G1 (slice
// G1-1), so that go mod tidy keeps them until the G1 packages import them.
// The build tag is never set, so nothing here is compiled into GoPMgr.
// Slice G1-2 removes this file when internal/interop/contract imports
// protobuf and gRPC; G1-4 imports go-winio.
package interop

import (
	_ "github.com/Microsoft/go-winio"
	_ "google.golang.org/grpc"
	_ "google.golang.org/protobuf/proto"
)
