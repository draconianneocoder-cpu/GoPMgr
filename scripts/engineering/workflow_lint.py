# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Check tracked workflow structure with verified actionlint, without execution."""
import argparse
import pathlib
import shutil
import subprocess
import sys

import provision
from shellcheck import ENV, ROOT


def workflows(root):
    git = ['/usr/bin/git', '-c', 'core.fsmonitor=false', '-C', str(root)]
    prefix = subprocess.run([*git, 'rev-parse', '--show-prefix'], env=ENV, capture_output=True, check=True)
    provision.require(prefix.stdout == b'\n', 'inventory root must be the repository root')
    result = subprocess.run([*git, 'ls-files', '--cached', '-z', '--', '.github/workflows'],
                            env=ENV, capture_output=True, check=True)
    names = [pathlib.Path(p.decode(sys.getfilesystemencoding(), errors='surrogateescape'))
             for p in result.stdout.split(b'\0') if p]
    provision.require(len(names) == len(set(names)), 'unmerged or duplicate workflow index paths')
    files = []
    for name in sorted(names):
        if name.suffix not in {'.yml', '.yaml'}:
            continue
        provision.require(name.parent == pathlib.Path('.github/workflows'), f'{name}: nested workflow is not supported')
        path = root / name
        provision.require(path.resolve() == path and not path.is_symlink(), f'{name}: symlink is not a workflow input')
        provision.require(path.is_file(), f'{name}: tracked workflow is missing or not a regular file')
        files.append(str(name))
    provision.require(files, 'no tracked workflows')
    return files


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=pathlib.Path, default=ROOT)
    parser.add_argument('--actionlint', type=pathlib.Path, help='explicit binary; defaults to actionlint on PATH')
    args = parser.parse_args()
    try:
        root = args.root.resolve()
        files = workflows(root)
        selected = args.actionlint or shutil.which('actionlint')
        provision.require(selected, 'actionlint missing; explicitly provision it and put it on PATH')
        binary = pathlib.Path(selected).resolve()
        artifact = provision.read_policy(ROOT)[2]['tools']['actionlint']
        provision.check_binary(binary, artifact['platforms'][provision.host_platform()]['executable_sha256'],
                               artifact['version'], '-version')
        # Explicit reviewed config and disabled sidecars make results independent
        # of ambient ignore rules or executable discovery on the caller's PATH.
        result = subprocess.run([str(binary), '-config-file', str(ROOT / '.github/actionlint.yaml'),
                                 '-shellcheck=', '-pyflakes=', '-no-color', '-oneline',
                                 *[str(root / name) for name in files]], cwd=root,
                                env={'PATH': '/usr/bin:/bin', 'LANG': 'C'},
                                capture_output=True, text=True, timeout=120)
        if result.stdout:
            print(result.stdout, end='')
        if result.stderr:
            print(result.stderr, end='', file=sys.stderr)
        if result.returncode:
            sys.exit(1)
        print(f'actionlint: {len(files)} tracked workflows; clean')
    except subprocess.CalledProcessError as error:
        detail = error.stderr.decode(errors='replace').strip() if isinstance(error.stderr, bytes) else error.stderr
        sys.exit(f'actionlint: Git inventory failed: {detail or error}')
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        sys.exit(f'actionlint: {error}')


if __name__ == '__main__':
    main()
