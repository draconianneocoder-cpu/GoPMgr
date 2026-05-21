<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: CC0-1.0
-->

# LICENSES

This directory holds the full text of every license that applies to files in
this repository, in the format required by the [REUSE
specification](https://reuse.software/spec/) so that `reuse lint` passes.

## How to populate this directory

Each file in the project carries an `SPDX-License-Identifier` tag. The
identifiers used in this project are:

| SPDX ID              | Applies to                                | Source                                                                                              |
| -------------------- | ----------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `GPL-3.0-or-later`   | Source code (Go, Svelte, TS, Makefile)    | https://www.gnu.org/licenses/gpl-3.0.txt                                                            |
| `GFDL-1.3-or-later`  | User-facing documentation (`docs/`)       | https://www.gnu.org/licenses/fdl-1.3.txt                                                            |
| `CC0-1.0`            | Tiny config files and this README         | https://creativecommons.org/publicdomain/zero/1.0/legalcode.txt                                     |
| `OFL-1.1`            | Bundled fonts (Liberation, Noto, Source Sans 3, JetBrains Mono) | https://openfontlicense.org/documents/OFL.txt                                  |
| `LicenseRef-Bitstream-Vera` | Bundled DejaVu Sans font           | https://dejavu-fonts.github.io/License.html                                                         |

The font binaries themselves are fetched by `scripts/fetch-fonts.sh`
(not committed); their licenses are declared in the top-level
`REUSE.toml` because binary files cannot carry inline SPDX headers.

Run the following once after cloning to drop the full license texts in place
(the REUSE tool can also do this with `reuse download --all`):

```sh
curl -L -o GPL-3.0-or-later.txt   https://www.gnu.org/licenses/gpl-3.0.txt
curl -L -o GFDL-1.3-or-later.txt  https://www.gnu.org/licenses/fdl-1.3.txt
curl -L -o CC0-1.0.txt            https://creativecommons.org/publicdomain/zero/1.0/legalcode.txt
curl -L -o OFL-1.1.txt            https://openfontlicense.org/documents/OFL.txt
curl -L -o LicenseRef-Bitstream-Vera.txt https://dejavu-fonts.github.io/License.html
```

Or, if you have the `reuse` tool installed:

```sh
pip install reuse
reuse download --all
```

The actual `.txt` files are intentionally not committed by the initial
scaffold because they are large and the project must point at the canonical
versions hosted by the FSF / Creative Commons.
