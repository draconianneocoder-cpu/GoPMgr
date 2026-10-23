<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: CC0-1.0
-->

# GoPMgr License

Copyright (C) 2026 James L. Burns and The GoPMgr Contributors.

GoPMgr is **free software**. Its source code is licensed under the **GNU
General Public License, version 3 or (at your option) any later version**
(`GPL-3.0-or-later`).

```
GoPMgr — a local-first desktop project-controls application.
Copyright (C) 2026 James L. Burns and The GoPMgr Contributors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
```

The complete text of the GPL version 3 is in
[`LICENSES/GPL-3.0-or-later.txt`](LICENSES/GPL-3.0-or-later.txt).

## What GPL-3.0-or-later means for you

- **Use.** You may run GoPMgr for any purpose, including commercial use.
- **Study and modify.** The complete corresponding source is this repository.
- **Share.** You may redistribute copies, modified or not.
- **Copyleft.** If you distribute GoPMgr or a derivative — including as a
  binary — you must do so under GPL-3.0-or-later (or a compatible later GPL
  version) and make the corresponding source available to your recipients.
- **No added restrictions.** You may not impose further legal terms that
  restrict the freedoms this license grants.
- **No warranty.** GoPMgr is provided without warranty of any kind, to the
  extent permitted by law.

This summary is for convenience only; the governing terms are those in
`LICENSES/GPL-3.0-or-later.txt`.

## This repository is not entirely GPL

GoPMgr follows the [REUSE](https://reuse.software/) specification: **every
file carries its own `SPDX-License-Identifier`**, and the authoritative
license of any file is the identifier in that file (or in
[`REUSE.toml`](REUSE.toml) for files that cannot hold an inline header).
The licenses that appear in this project are:

| SPDX identifier             | Applies to                                                              |
| --------------------------- | ----------------------------------------------------------------------- |
| `GPL-3.0-or-later`          | Source code — Go, Svelte, TypeScript, Makefile, build/config scaffolds  |
| `GFDL-1.3-or-later`         | User-facing documentation (`README.md`, `docs/`, and most `*.md`)       |
| `CC0-1.0`                   | Tiny config files, these license notes, and the compact sRGB ICC profile |
| `CC-BY-SA-4.0`              | OWASP Cornucopia card text and GoPMgr's curation of it (`.claude/skills/cornucopia-risk-cards/references/`) |
| `OFL-1.1`                   | The committed IBM Plex fonts (Sans, Serif, Mono, Sans Condensed; TrueType for PDFs and WOFF2 for the app) and Source Sans 3; also the optional downloaded fonts Liberation, Noto Sans, JetBrains Mono, Roboto, Arimo, Cousine, and Ledger |
| `Bitstream-Vera`            | The optional downloaded DejaVu Sans font                                 |
| `MIT`                       | Wails-derived Windows installer templates (`build/windows/`) and the vendored `go-sqlcipher` Go code (`third_party/go-sqlcipher/`) |
| `BSD-3-Clause`, `blessing`, `Unlicense` | Other vendored code in `third_party/go-sqlcipher/`: the SQLCipher and SQLite sources, LibTomCrypt, and one Go Authors file |

The full text of every license that applies to a committed file, including
`OFL-1.1` for the committed IBM Plex and Source Sans 3 fonts, lives in the
[`LICENSES/`](LICENSES/) directory, as REUSE requires; the fonts' copyright
notices and license also ship inside the app, in
`internal/fonts/assets/FONT-NOTICES.txt` and next to the app's web fonts. The
other font families are optional: `scripts/fetch-fonts.sh` (`make fonts`)
downloads them for local builds, they are **not committed**, and CI and
release builds never download them, so release binaries embed only IBM Plex
and Source Sans 3. Their licenses
(`OFL-1.1`, and `Bitstream-Vera` for DejaVu Sans) are recorded above, in
`REUSE.toml`, and in
[`internal/fonts/assets/README.md`](internal/fonts/assets/README.md), which
also links each family's upstream license file. The script downloads font
files only, not license texts.

For the rationale behind each identifier and how to regenerate the license
texts, see [`LICENSES.md`](LICENSES.md).

## Third-party dependencies

GoPMgr links Go modules and npm packages that carry their own permissive or
copyleft licenses (for example MIT, BSD-3-Clause, Apache-2.0, and MPL-2.0).
Those licenses are compatible with distributing GoPMgr under
GPL-3.0-or-later. See [`DEPENDENCIES.md`](docs/DEPENDENCIES.md) for the dependency
policy and the tools used to audit license compatibility.

## Verifying compliance

REUSE compliance is enforced in CI and can be checked locally:

```sh
make license-check   # runs `reuse lint`
```

A passing `reuse lint` confirms that every file has a known license and that
every referenced license text is present in `LICENSES/`.
