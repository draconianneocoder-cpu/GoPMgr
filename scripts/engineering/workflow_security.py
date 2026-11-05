# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Enforce the approved local offline workflow security policy without writes."""
import argparse
import json
import pathlib
import shutil
import subprocess
import sys

import provision
from workflow_lint import ROOT, workflows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=pathlib.Path, default=ROOT)
    parser.add_argument('--zizmor', type=pathlib.Path, help='explicit binary; defaults to zizmor on PATH')
    args = parser.parse_args()
    try:
        root = args.root.resolve()
        files = workflows(root)
        selected = args.zizmor or shutil.which('zizmor')
        provision.require(selected, 'zizmor missing; explicitly provision it and put it on PATH')
        binary = pathlib.Path(selected).resolve()
        artifact = provision.read_policy(ROOT)[2]['tools']['zizmor']
        provision.check_binary(binary, artifact['platforms'][provision.host_platform()]['executable_sha256'],
                               artifact['version'], '--version')
        paths = [str(root / name) for name in files]
        result = subprocess.run([str(binary), '--offline', '--strict-collection', '--persona=regular',
                                 '--no-config', '--no-ignores', '--no-progress', '--format=json', *paths],
                                cwd=root, env={'PATH': '/usr/bin:/bin', 'LANG': 'C'},
                                capture_output=True, text=True, timeout=120)
        provision.require(result.returncode in {0, 11, 12, 13, 14},
                          f'zizmor failed (exit {result.returncode}): {result.stderr.strip()}')
        findings = json.loads(result.stdout)
        provision.require(isinstance(findings, list), 'unexpected zizmor JSON: expected finding list')
        provision.require(result.returncode == 0 or findings, 'zizmor reported findings without finding data')
        for finding in findings:
            try:
                ident = finding['ident']
                severity = finding['determinations']['severity']
                confidence = finding['determinations']['confidence']
                path = finding['locations'][0]['symbolic']['key']['Local']['verbatim_path']
                ignored = finding['ignored']
            except (KeyError, TypeError, IndexError) as error:
                raise ValueError('unexpected zizmor finding schema') from error
            provision.require(isinstance(ident, str) and ident and path in paths and isinstance(ignored, bool)
                              and severity in ('Informational', 'Low', 'Medium', 'High')
                              and confidence in ('Low', 'Medium', 'High'), 'unexpected zizmor finding values')
            print(f'{json.dumps(path)}: {ident}: {severity}/{confidence}')
        print(f'zizmor: {len(files)} tracked workflows; {len(findings)} findings')
        if findings:
            sys.exit(1)
    except subprocess.CalledProcessError as error:
        detail = error.stderr.decode(errors='replace').strip() if isinstance(error.stderr, bytes) else error.stderr
        sys.exit(f'workflow-security: Git inventory failed: {detail or error}')
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        sys.exit(f'workflow-security: {error}')


if __name__ == '__main__':
    main()
