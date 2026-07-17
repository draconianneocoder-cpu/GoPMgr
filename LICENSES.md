<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
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
| `GFDL-1.3-or-later`  | User-facing documentation (`docs/`)       | https://www.gnu.org/licenses/fdl-1.3.txt                                                            |
| `CC0-1.0`            | Tiny config files, license notes, and compact ICC profile | https://creativecommons.org/publicdomain/zero/1.0/legalcode.txt                  |
| `OFL-1.1`            | Bundled fonts (Liberation, Noto, Source Sans 3, JetBrains Mono, Ledger) | https://openfontlicense.org/documents/OFL.txt                          |
| `LicenseRef-Bitstream-Vera` | Bundled DejaVu Sans font           | https://dejavu-fonts.github.io/License.html                                                         |
| `Apache-2.0`         | Bundled fonts (Roboto, Arimo, Cousine)    | https://www.apache.org/licenses/LICENSE-2.0.txt                                                     |

The font binaries themselves are fetched by `scripts/fetch-fonts.sh`
(not committed) and ignored as local downloads. Their upstream licenses are
documented here and in `internal/fonts/assets/README.md`. The compact sRGB ICC
profile used for PDF/A-3 OutputIntent embedding is committed because it is a
small build input and the Go embed directive needs it in clean checkouts.

Run the following after adding a new SPDX identifier:

```sh
curl -L -o GPL-3.0-or-later.txt   https://www.gnu.org/licenses/gpl-3.0.txt
curl -L -o GFDL-1.3-or-later.txt  https://www.gnu.org/licenses/fdl-1.3.txt
curl -L -o CC0-1.0.txt            https://creativecommons.org/publicdomain/zero/1.0/legalcode.txt
curl -L -o OFL-1.1.txt            https://openfontlicense.org/documents/OFL.txt
curl -L -o LicenseRef-Bitstream-Vera.txt https://dejavu-fonts.github.io/License.html
curl -L -o Apache-2.0.txt         https://www.apache.org/licenses/LICENSE-2.0.txt
```

Or, if you have the `reuse` tool installed:

```sh
pip install reuse
reuse download --all
```

License text files that correspond to committed assets should be tracked so
`reuse lint` can run without network access. License texts for ignored local
downloads can be regenerated locally when those assets are audited.
