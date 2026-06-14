// SPDX-FileCopyrightText: 2026 The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

module pmforge

go 1.26.4

require (
	github.com/digitorus/pkcs7 v0.0.0-20250730155240-ffadbf3f398c
	github.com/gomutex/godocx v0.1.5
	// V2.x — Foundation Slice additions:
	// github.com/gorules/zen (via its Go binding zen-go) drives the
	// Launchpad's template-seeding rules as JDM (JSON Decision Model)
	// data rather than a Go switch. New industry/methodology combos
	// are one row in launchpad_seeds.json. Used in internal/templates.
	github.com/gorules/zen-go v0.20.0
	github.com/jung-kurt/gofpdf v1.16.2

	// rickar/cal/v2 supplies maintained holiday datasets for ~40
	// countries. Used by internal/calendar and internal/export/ical.go
	// to mark holidays on the Timeline view and to skip non-business
	// days when emitting iCal events.
	github.com/rickar/cal/v2 v2.1.16
	github.com/wailsapp/wails/v2 v2.9.2
	github.com/xuri/excelize/v2 v2.8.1
	golang.org/x/crypto v0.31.0
)

require (
	github.com/mutecomm/go-sqlcipher/v4 v4.4.2
	gonum.org/v1/gonum v0.17.0
)

require (
	github.com/bep/debounce v1.2.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/jchv/go-winloader v0.0.0-20210711035445-715c2860da7e // indirect
	github.com/labstack/echo/v4 v4.10.2 // indirect
	github.com/labstack/gommon v0.4.0 // indirect
	github.com/leaanthony/go-ansi-parser v1.6.0 // indirect
	github.com/leaanthony/gosod v1.0.3 // indirect
	github.com/leaanthony/slicer v1.6.0 // indirect
	github.com/leaanthony/u v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/richardlehane/mscfb v1.0.4 // indirect
	github.com/richardlehane/msoleps v1.0.3 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/samber/lo v1.38.1 // indirect
	github.com/tidwall/gjson v1.17.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tkrajina/go-reflector v0.5.6 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/wailsapp/go-webview2 v1.0.16 // indirect
	github.com/wailsapp/mimetype v1.4.1 // indirect
	github.com/xuri/efp v0.0.0-20231025114914-d1ff6096ae53 // indirect
	github.com/xuri/nfp v0.0.0-20230919160717-d98342af3f05 // indirect
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.23.0 // indirect
)
