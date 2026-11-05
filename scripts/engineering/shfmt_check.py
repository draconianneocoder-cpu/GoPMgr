# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Check tracked shell formatting without rewriting or executing sources."""
import argparse
import json
import pathlib
import shutil
import subprocess
import sys

import provision
from shellcheck import ROOT, tracked_inventory


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=pathlib.Path, default=ROOT)
    parser.add_argument('--shfmt', type=pathlib.Path, help='explicit binary; defaults to shfmt on PATH')
    args = parser.parse_args()
    try:
        root = args.root.resolve()
        inventory = tracked_inventory(root)
        selected = args.shfmt or shutil.which('shfmt')
        provision.require(selected, 'shfmt missing; explicitly provision it and put it on PATH')
        binary = pathlib.Path(selected).resolve()
        artifact = provision.read_policy(ROOT)[2]['tools']['shfmt']
        provision.check_binary(binary, artifact['platforms'][provision.host_platform()]['executable_sha256'],
                               artifact['version'], '--version')
        drift = 0
        for item in inventory['files']:
            name = item['path']
            dialect = 'posix' if item['shell'] == 'sh' else 'bash'
            source = (root / name).read_bytes()
            # Explicit parser/printer options disable EditorConfig. Stdin avoids
            # path-option ambiguity and gives the formatter no source write path.
            try:
                result = subprocess.run([str(binary), '-i=0', '-ln=' + dialect],
                                        input=source, capture_output=True, timeout=30,
                                        env={'PATH': '/usr/bin:/bin', 'LANG': 'C'})
            except subprocess.TimeoutExpired as error:
                raise ValueError(f'{json.dumps(name)}: shfmt timed out after 30 seconds') from error
            provision.require(result.returncode == 0,
                              f'{json.dumps(name)}: shfmt failed: {result.stderr.decode(errors="replace").strip()}')
            if result.stdout != source:
                print(f'{json.dumps(name)}: needs formatting')
                drift += 1
        print(f'shfmt: {len(inventory["files"])} tracked scripts; {drift} need formatting')
        if drift:
            sys.exit(1)
    except subprocess.CalledProcessError as error:
        detail = error.stderr.decode(errors='replace').strip() if isinstance(error.stderr, bytes) else error.stderr
        sys.exit(f'shfmt: Git inventory failed: {detail or error}')
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        sys.exit(f'shfmt: {error}')


if __name__ == '__main__':
    main()
