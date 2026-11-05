// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

module gopmgr

go 1.27.3

// SQLCipher 4.4.2 with WAL-reset and transaction-cleanup corrections.
// Provenance and maintenance instructions: third_party/go-sqlcipher/GOPMGR.md.
replace github.com/mutecomm/go-sqlcipher/v4 => ./third_party/go-sqlcipher

require (
	github.com/Microsoft/go-winio v0.6.4
	github.com/apache/arrow-go/v18 v18.5.1
	github.com/digitorus/pkcs7 v0.0.0-20250730155240-ffadbf3f398c
	github.com/digitorus/timestamp v0.0.0-20250524132541-c45532741eea
	github.com/go-pdf/fpdf v0.9.0
	github.com/gomutex/godocx v0.1.5
	// V2.x — Foundation Slice additions:
	// github.com/gorules/zen (via its Go binding zen-go) drives the
	// Launchpad's template-seeding rules as JDM (JSON Decision Model)
	// data rather than a Go switch. New industry/methodology combos
	// are one row in launchpad_seeds.json. Used in internal/templates.
	github.com/gorules/zen-go v0.20.0
	github.com/mutecomm/go-sqlcipher/v4 v4.4.2
	github.com/pelletier/go-toml/v2 v2.4.3

	// rickar/cal/v2 supplies maintained holiday datasets for ~40
	// countries. Used by internal/calendar and internal/export/ical.go
	// to mark holidays on the Timeline view and to skip non-business
	// days when emitting iCal events.
	github.com/rickar/cal/v2 v2.1.32
	github.com/wailsapp/wails/v3 v3.1.0
	github.com/xuri/excelize/v2 v2.11.1-0.20261003002531-6258dcebc4e2
	golang.org/x/crypto v0.58.1-0.20261009181718-2f1f834330b2
	golang.org/x/mod v0.42.0
	golang.org/x/sys v0.49.0
	gonum.org/v1/gonum v0.17.0
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/apache/thrift v0.24.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/asmfmt v1.3.2 // indirect
	github.com/klauspost/compress v1.18.7 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/minio/asm2plan9s v0.0.0-20200509001527-cdd76441f9d8 // indirect
	github.com/minio/c2goasm v0.0.0-20190812172519-36a3d3bbc4f3 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pierrec/lz4/v4 v4.1.25 // indirect
	github.com/richardlehane/mscfb v1.0.8 // indirect
	github.com/richardlehane/msoleps v1.0.6 // indirect
	github.com/tidwall/gjson v1.17.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tiendc/go-deepcopy v1.7.2 // indirect
	github.com/xuri/efp v0.0.2 // indirect
	github.com/xuri/nfp v0.0.2-0.20250530014748-2ddeb826f9a9 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	golang.org/x/exp v0.0.0-20260410095643-746e56fc9e2f // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/telemetry v0.0.0-20260924152758-ed294f943157 // indirect
	golang.org/x/term v0.47.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/tools v0.51.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260817212433-ac3dfec99bb1 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
)
