<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: CC0-1.0
-->

# License Notes

> For the project's headline license statement — what GPL-3.0-or-later grants
> you and how the repository's mixed licensing fits together — see
> [LICENSE.md](LICENSE.md). This file documents how the `LICENSES/` directory
> is populated and maintained.

The `LICENSES/` directory holds the full text of every license that applies to
files in this repository, in the format required by the [REUSE
specification](https://reuse.software/spec/).

## How to populate this directory

Each file in the project carries an `SPDX-License-Identifier` tag. The
identifiers used in this project are:

| SPDX ID              | Applies to                                | Source                                                                                              |
| -------------------- | ----------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `GPL-3.0-or-later`   | Source code (Go, Svelte, TS, Makefile)    | https://www.gnu.org/licenses/gpl-3.0.txt                                                            |
| `GPL-3.0-or-later`   | GoPMgr-owned Bobby Beaver branding artwork | [docs/branding.md](docs/branding.md)                                                                |
| `GFDL-1.3-or-later`  | User-facing and agent-facing documentation (`docs/`, root guides such as `AGENTS.md`, `.claude/skills/`) | https://www.gnu.org/licenses/fdl-1.3.txt                                                            |
| `CC-BY-SA-4.0`       | Third-party OWASP Cornucopia card text and GoPMgr's derived curation of it (`.claude/skills/cornucopia-risk-cards/references/`) | https://creativecommons.org/licenses/by-sa/4.0/legalcode.txt                                        |
| `CC0-1.0`            | Tiny config files, license notes, and compact ICC profile | https://creativecommons.org/publicdomain/zero/1.0/legalcode.txt                  |
| `OFL-1.1`            | Committed IBM Plex fonts (Sans, Serif, Mono, Sans Condensed, Sans JP, KR, SC, Arabic, Hebrew, Devanagari, Thai) and Source Sans 3; optional downloaded fonts (Liberation, Noto Sans, JetBrains Mono, Roboto, Arimo, Cousine, Ledger) | https://raw.githubusercontent.com/spdx/license-list-data/main/text/OFL-1.1.txt |
| `Bitstream-Vera`     | Optional downloaded DejaVu Sans font      | https://raw.githubusercontent.com/spdx/license-list-data/main/text/Bitstream-Vera.txt |
| `MIT`                | Wails-derived Windows installer templates; vendored `go-sqlcipher` Go code (`third_party/go-sqlcipher/`) | https://github.com/wailsapp/wails/blob/v2.13.0/LICENSE                                             |
| `BSD-3-Clause`, `blessing`, `Unlicense` | Other vendored code in `third_party/go-sqlcipher/` (SQLCipher and SQLite sources, LibTomCrypt, one Go Authors file) | SPDX License List, via `reuse download` |
| `LicenseRef-Third-Party-Notices` | The generated `third_party/GO-MODULE-NOTICES.txt`, whose sections keep their own terms (see [Go module notices](#go-module-notices-in-release-packages)) | Written for GoPMgr; `LICENSES/LicenseRef-Third-Party-Notices.txt` |

The IBM Plex fonts (the default typeface and PDF/A baseline), Source Sans 3,
and their `OFL-1.1` license text are committed because clean release builds
must embed those fonts without a network fetch.
Other font binaries are fetched by `scripts/fetch-fonts.sh` and ignored as
optional local downloads; CI and release builds never fetch them. Their
licenses are documented here and in `internal/fonts/assets/README.md`, which
also lists each family's version, upstream copyright notice, and upstream
license file. Roboto, Arimo, and Cousine were once `Apache-2.0`; the pinned
upstream releases are `OFL-1.1`. DejaVu's upstream `LICENSE` also contains the
Arev Fonts license, which applies the same conditions as `Bitstream-Vera` to
its Arev glyphs. The compact sRGB ICC profile used for
PDF/A OutputIntent embedding is likewise committed because the Go embed
directive needs it in clean checkouts.

Run the following after adding a new SPDX identifier:

```sh
curl -L -o GPL-3.0-or-later.txt   https://www.gnu.org/licenses/gpl-3.0.txt
curl -L -o GFDL-1.3-or-later.txt  https://www.gnu.org/licenses/fdl-1.3.txt
curl -L -o CC0-1.0.txt            https://creativecommons.org/publicdomain/zero/1.0/legalcode.txt
curl -L -o OFL-1.1.txt            https://raw.githubusercontent.com/spdx/license-list-data/main/text/OFL-1.1.txt
curl -L -o Bitstream-Vera.txt     https://raw.githubusercontent.com/spdx/license-list-data/main/text/Bitstream-Vera.txt
curl -L -o MIT.txt                https://raw.githubusercontent.com/wailsapp/wails/v2.13.0/LICENSE
curl -L -o CC-BY-SA-4.0.txt       https://creativecommons.org/licenses/by-sa/4.0/legalcode.txt
```

Or, if you have the `reuse` tool installed:

```sh
pipx install 'reuse[charset-normalizer]==6.2.0'
reuse download --all
```

This is the same command `scripts/check-release.sh` and both GitHub Actions
workflows use, and the pin matters for the same reason they pin it: a local
`reuse lint` pass only implies the 6.2.0 gate passes if it ran on 6.2.0. The
`charset-normalizer` extra is required rather than optional — `reuse` depends
unconditionally on `python-magic`, a ctypes binding to the native `libmagic`
library that pip does not ship, and aborts at import time when no encoding
module loads, and macOS has no `libmagic`. `pipx` rather than `pip` because a
Homebrew or other PEP 668 "externally managed" interpreter — the macOS case
this paragraph is about — refuses `pip install` outright, before the extra
ever matters.

License text files that correspond to committed assets should be tracked so
`reuse lint` can run without network access. License texts for ignored local
downloads can be regenerated locally when those assets are audited.

## Go module notices in release packages

Every release binary links the Go standard library and dozens of Go modules.
Their licenses (MIT, BSD, Apache-2.0, ISC and others) require the copyright
notice and license text to travel with the binary. GoPMgr collects them in one
generated file, `third_party/GO-MODULE-NOTICES.txt`, and every package ships
it:

| Package | Built by | Installed as |
| --- | --- | --- |
| Tarball | `scripts/package.sh` | `LICENSES/go-modules.txt` |
| `.deb` and `.rpm` | `scripts/package-linux.sh`, `build/linux/nfpm.yaml` | `/usr/share/doc/gopmgr/go-module-notices.txt` |
| Debian source package | `debian/rules` | `/usr/share/doc/gopmgr/go-module-notices.txt` |
| macOS `.dmg` and `.pkg` | `scripts/package-macos.sh`, `scripts/package-macos-installer.sh` | `Contents/Resources/Licenses/go-modules.txt` inside the app |
| Windows installer | `build/windows/installer/project.nsi` | `licenses\go-modules.txt` in the install directory |

### How the file is made

`tools/module-notices` runs `go list -deps` on the root package for each
release target in `.github/workflows/release.yml` (darwin/arm64, linux/amd64,
windows/amd64), with the tags `wails build` uses. For every linked module it
copies the license-like files (`LICENSE*`, `COPYING*`, `NOTICE*`, `PATENTS`
and similar) from the module root and from each directory between the root and
a linked package. Those are the places `go mod vendor` takes license
metadata from. It also copies the files of a `THIRD_PARTY_LICENSES` or
`LICENSES` directory found there. The texts come unchanged from the module
cache or from a local replacement such as `third_party/go-sqlcipher`; only line
endings and trailing spaces are normalized. The tool fails on a module with no
license file, so a new dependency without one cannot ship unnoticed. It needs
only the Go standard library.

### When to regenerate

Run `make module-notices` and commit the result whenever `go.mod` changes,
including Dependabot updates. Review the diff: a changed license text is a
license change.

Three checks keep the file honest:

- `make module-notices-check`, part of `make verify`, fails when the file no
  longer matches what the release targets link.
- `scripts/verify-module-notices.sh` runs at packaging time on the binary that
  ships. It reads the binary's own module list (`go version -m`) and fails
  unless the file has a section for the standard library and for every module
  at the exact version linked. `make module-notices-tests` covers it.
- The packaging tests check that each package carries an exact copy:
  `scripts/package-linux_test.sh`, `scripts/package-macos_test.sh`,
  `scripts/package-macos-installer_test.sh` and
  `scripts/validate-windows-nsis-template.sh`.

### REUSE metadata

The file reproduces third-party texts whose terms differ section by section,
and some of them carry their own SPDX tags. `REUSE.toml` therefore annotates
it with `precedence = "override"` and `LicenseRef-Third-Party-Notices`, which
says that each reproduced text keeps its own terms.

### Not covered yet

- Native libraries that a Go module ships in compiled form can carry notices
  of their own that the module does not include. The DuckDB binding modules
  (`github.com/duckdb/duckdb-go-bindings/lib/*`) are the known case: their
  static libraries bundle code such as ICU, RE2, zstd and Parquet components,
  and the module ships only DuckDB's MIT license.
- The frontend's npm packages, bundled into `frontend/dist` and embedded in
  the binary, have no generated notices file yet.
