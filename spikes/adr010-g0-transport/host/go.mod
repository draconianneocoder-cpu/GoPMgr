// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// ADR-010 G0 transport spike host. A separate module, so nothing here
// reaches GoPMgr's production go.mod (OWN-11).
module gopmgr-g0spike/host

go 1.27.3

require (
	github.com/Microsoft/go-winio v0.6.4
	golang.org/x/sys v0.49.0
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

require (
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
)
