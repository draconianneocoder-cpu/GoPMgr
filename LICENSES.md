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

## DuckDB native library notices

The DuckDB binding modules (`github.com/duckdb/duckdb-go-bindings/lib/*`) ship
DuckDB as prebuilt static libraries, and the module itself carries only
DuckDB's MIT license. The libraries also contain third-party code: ICU, RE2,
Zstandard, Mbed TLS, the Parquet stack (Thrift, Snappy, Brotli, LZ4), and
more. `third_party/DUCKDB-NATIVE-NOTICES.txt` reproduces their license and
notice texts. The TPC-H and TPC-DS generators the modules also carry are
excluded from GoPMgr; see [below](#excluded-the-tpc-h-and-tpc-ds-generators). It ships next to the Go module
notices: `LICENSES/duckdb-native.txt` in the tarball,
`/usr/share/doc/gopmgr/duckdb-native-notices.txt` in the Linux packages,
`Contents/Resources/Licenses/duckdb-native.txt` in the macOS app, and
`licenses\duckdb-native.txt` on Windows.

### How the file is made

`tools/duckdb-notices` holds a reviewed list of the components. Each entry
names the linked library that contains it and the license file, or the
license comment lines, in the DuckDB source. The tool reads those texts from
the DuckDB source the libraries were built from. It checks the checkout's
commit against its pin and checks both tracked working files and the index
for modifications first.

The pins for duckdb-go-bindings v0.10506.0:

- the module's `Makefile` builds DuckDB v1.5.6;
- the tag is commit `069cc9f9b5be802405797faecc284961b07c70ef`, which matches
  the source ID compiled into `libduckdb_static.a`;
- robin-hood-hashing 3.11.5 supplies the MIT text that DuckDB credits in
  `src/common/checksum.cpp` but does not carry.

The list was built from the symbols of every linked library on all three
platforms (`llvm-nm`, matched against each component's namespace), plus a
scan of DuckDB's own sources for embedded third-party code (nanoarrow and
ADBC, ALP, the merge-sort tree, MD5, robin-hood-hashing). Every component is
linked on all three platforms, except jemalloc, which is built into the Linux
library only. Snowball, Catch and the IMDB data are in the DuckDB tree but
not in the linked libraries, so they are not listed.

### When to update

- `make duckdb-notices-check`, part of `make verify`, works offline once the
  modules are downloaded. It fails when `go.mod` selects any
  duckdb-go-bindings module at a version other than the reviewed one, when
  the full notice-file digest differs, a reviewed module inventory changes,
  or the exclusion audit cannot complete (see below).
- On a DuckDB update, repeat the review for the new release: link flags,
  library symbols, and new `third_party` directories. Then update the pins and
  the component list in `tools/duckdb-notices`, and run `make duckdb-notices`.
  That target clones the pinned sources and rewrites the file. Its unit test
  also fails if a linked library is not accounted for.
- `scripts/verify-module-notices.sh` refuses to package a binary that links a
  DuckDB library module the file does not name at that exact version. It also
  invokes the native input/digest check; coverage headers alone do not qualify
  a notice file.

### Excluded: the TPC-H and TPC-DS generators

Each platform module also ships DuckDB's TPC-H and TPC-DS data generators as
separate archives, `libtpch_extension.a` and `libtpcds_extension.a`, and
names them on its link line. The TPC-H generator (`dbgen`) is under the TPC
End User License Agreement 2.2, not an open-source license. DuckDB's copy of
the TPC-DS generator (`dsdgen`) has no license file; its sources carry a TPC
legal notice. Whether that code can be distributed with GoPMgr under
GPL-3.0-or-later has not had a qualified legal review, so the maintainer has
excluded it from every GoPMgr distribution, on all platforms and in all
formats (decision of 2026-10-26; see
[the decision record](docs/review/TPC-benchmark-exclusion-decision.md)).

The reviewed prebuilt libraries configure both extensions with `DONT_LINK`.
The fixed-input audit finds no references to their externally defined symbols
in other linked archives or Go cgo selectors. This supports normal static-link
exclusion but is not a universal absence proof. Historical marker-scan results
and their limits are preserved in the decision record. Bare autoload names
such as `dbgen`, `dsdgen`, `tpch` and `tpcds` are not exclusion markers.

The exclusion is enforced, and each check is release-blocking:

- `third_party/EXCLUDED-NATIVE-CODE.txt` lists the two archives and eight
  reviewed marker strings (class names and error messages). Absence of those
  markers is bounded evidence for the reviewed inputs.
- `make duckdb-notices-check` audits every DuckDB platform module `go.mod`
  requires. It fails if another linked library or a cgo call in the Go
  bindings references a symbol the archives define, if inputs or supported
  link flags differ from the review, if required symbol evidence is missing,
  if a marker is missing from the archives, or if a marker appears in any other linked library.
- `scripts/verify-excluded-native-code.sh` fails on a binary that contains
  any marker, or on scan/tool errors. `scripts/verify-module-notices.sh` runs
  it, so every packaging path (tarball, `.deb` and `.rpm`, macOS app and installer, Windows
  installer, Debian source build) checks the binary it ships.
- `debian/rules` runs it with `--tree vendor` before building. `go mod
  vendor` copies the excluded archives into `vendor/`, so a plain vendored
  Debian build fails by design. This build hook does not prevent source-only
  archive creation (`dpkg-source -b`); publishing such an archive would
  distribute the TPC object code and remains prohibited. No vendored layout
  without them or source-publication workflow is approved here.

The notices file omits the TPC agreement under the exclusion policy. Passing
these technical checks does not establish license compliance or authorize a
release. See the [qualification procedure](docs/testing/duckdb-notices-qualification.md)
for reviewed input pins, full-source reproduction and evidence limits.

## Frontend package notices

The interface is a Vite bundle (`frontend/dist`) embedded in the binary.
`third_party/FRONTEND-NOTICES.txt` reproduces the license and notice files of
every npm package whose code is in that bundle. It ships next to the other
notices:

- `LICENSES/frontend.txt` in the tarball;
- `/usr/share/doc/gopmgr/frontend-notices.txt` in the Linux packages;
- `Contents/Resources/Licenses/frontend.txt` in the macOS app;
- `licenses\frontend.txt` on Windows.

### How the file is made

`frontend/scripts/bundle-notices.mjs` runs the real production build
(`vite.config.ts`) in memory, without writing `dist/`. It takes the package
set from three places, so dev-only tooling that leaves no code behind is not
listed:

- the bundle's own module graph;
- the helper modules Vite injects;
- license banners in generated assets, such as Tailwind's in the stylesheet.

It fails on a module it cannot place, an unknown injected module, or a
package with no license file. It needs only Node and the frontend's existing
dev dependencies.

Two reviewed exceptions are recorded in `frontend/scripts/bundle-notices-lib.mjs`:

- **`worker-f`**, a dependency of read-excel-file, ships no license file.
  Neither does its source at the published tag. Its `package.json` declares
  MIT and names its author. The file therefore gives the standard MIT terms
  with that author as copyright holder, and says so. The entry is pinned to
  the reviewed version.
- **Vite's `LICENSE.md`** also lists the dependencies bundled into Vite's
  own Node.js build. Only its core license covers the two helpers Vite
  injects into the bundle, so the file keeps that section and says why.

### When to regenerate

- Run `make frontend-notices` after `npm ci` whenever `frontend/package-lock.json`
  changes, or when the app starts importing a new package. Commit the result.
- `make frontend-notices-check`, part of `make verify`, fails when the file no
  longer matches the bundle.
- `make frontend-notices-tests` unit-tests the generator with Node's built-in
  test runner.
