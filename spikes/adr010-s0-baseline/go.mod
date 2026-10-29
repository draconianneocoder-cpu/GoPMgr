// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// ADR-010 S0 baseline harness. A separate module that imports GoPMgr's
// packages read-only to measure today's behavior; nothing here reaches
// GoPMgr's go.mod or release.
module gopmgr/spikes/adr010-s0-baseline

go 1.27.3

require (
	golang.org/x/sys v0.49.0
	gopmgr v0.0.0
)

require (
	github.com/digitorus/pkcs7 v0.0.0-20250730155240-ffadbf3f398c // indirect
	github.com/digitorus/timestamp v0.0.0-20250524132541-c45532741eea // indirect
	github.com/go-pdf/fpdf v0.9.0 // indirect
	github.com/gomutex/godocx v0.1.5 // indirect
	github.com/mutecomm/go-sqlcipher/v4 v4.4.2 // indirect
	github.com/richardlehane/mscfb v1.0.8 // indirect
	github.com/richardlehane/msoleps v1.0.6 // indirect
	github.com/rickar/cal/v2 v2.1.32 // indirect
	github.com/tiendc/go-deepcopy v1.7.2 // indirect
	github.com/xuri/efp v0.0.2 // indirect
	github.com/xuri/excelize/v2 v2.11.1-0.20261003002531-6258dcebc4e2 // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/net v0.60.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
)

replace gopmgr => ../..

// GoPMgr's own replace directive does not apply to dependents, so it is
// repeated here.
replace github.com/mutecomm/go-sqlcipher/v4 => ../../third_party/go-sqlcipher
