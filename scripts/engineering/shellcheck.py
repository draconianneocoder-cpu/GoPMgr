# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Analyze tracked first-party shells without installation or source mutation."""
import argparse
import json
import os
import pathlib
import shlex
import shutil
import subprocess
import sys

import provision

ROOT = pathlib.Path(__file__).resolve().parents[2]
ENV = {'PATH': '/usr/bin:/bin', 'LANG': 'C', 'GIT_CONFIG_NOSYSTEM': '1',
       'GIT_CONFIG_GLOBAL': '/dev/null', 'GIT_OPTIONAL_LOCKS': '0'}
LIBRARIES = {'scripts/package-version-lib.sh', 'scripts/pades-lock.sh',
             'scripts/pades-publish.sh', 'scripts/validate-pdfa-lib.sh'}
EXCLUDED = {'vendor', 'third_party', 'node_modules'}
GENERATED = ('frontend/bindings/', 'frontend/dist/', 'build/bin/', 'build/packages/')


def tracked_inventory(root):
    git = ['/usr/bin/git', '-c', 'core.fsmonitor=false', '-C', str(root)]
    prefix = subprocess.run([*git, 'rev-parse', '--show-prefix'], env=ENV, capture_output=True, check=True)
    provision.require(prefix.stdout == b'\n', 'inventory root must be the repository root')
    result = subprocess.run([*git, 'ls-files', '--cached', '-z'], env=ENV, capture_output=True, check=True)
    paths = [os.fsdecode(p) for p in result.stdout.split(b'\0') if p]
    provision.require(len(paths) == len(set(paths)), 'unmerged or duplicate index paths')
    files, excluded = [], []
    for name in sorted(paths):
        path = root / name
        if EXCLUDED.intersection(pathlib.PurePosixPath(name).parts) or name.startswith(GENERATED):
            if path.suffix in {'.sh', '.bash', '.zsh'}:
                excluded.append({'path': name, 'reason': 'third-party or generated boundary'})
            continue
        provision.require(path.resolve() == path and not path.is_symlink(), f'{name}: symlink is not an analyzable first-party input')
        with path.open('rb') as source:
            header = source.readline(512).decode('utf-8', errors='replace').strip()
        tokens = shlex.split(header[2:]) if header.startswith('#!') else []
        program = pathlib.PurePosixPath(tokens[0]).name if tokens else ''
        if program == 'env':
            arguments = tokens[1:]
            if arguments and arguments[0] == '-S':
                arguments = arguments[1:]
            provision.require(arguments and not arguments[0].startswith('-') and '=' not in arguments[0],
                              f'{name}: unsupported env shebang {header!r}')
            program = pathlib.PurePosixPath(arguments[0]).name
        if path.suffix not in {'.sh', '.bash', '.zsh'} and program not in {'sh', 'bash', 'zsh', 'dash', 'ksh', 'fish'}:
            continue
        provision.require(program in {'bash', 'sh'}, f'{name}: unsupported shell declaration {header!r}')
        files.append({'path': name, 'shell': program})
    provision.require(files, 'no tracked first-party shell scripts')
    return {'files': files, 'excluded': excluded}


def scan(root, binary, inventory):
    findings = {}
    for dialect in ['bash', 'sh']:
        paths = [item['path'] for item in inventory['files'] if item['shell'] == dialect]
        libraries = sorted(LIBRARIES.intersection(paths)) if dialect == 'bash' else []
        flags = [str(binary), '--norc', '--check-sourced', '--source-path=SCRIPTDIR',
                 '--format=json', '--shell=' + dialect, '--']
        for path in paths:
            # At most five operands. Other candidates cannot incidentally become
            # authorized sources because they happen to share an argument batch.
            operands = list(dict.fromkeys([path, *libraries]))
            result = subprocess.run([*flags, *operands], cwd=root,
                                    env={'PATH': '/usr/bin:/bin', 'LANG': 'C'},
                                    capture_output=True, text=True, timeout=120)
            provision.require(result.returncode in {0, 1}, f'ShellCheck failed for {path}: {result.stderr.strip()}')
            reports = json.loads(result.stdout)
            provision.require(result.returncode == 0 or reports, 'ShellCheck failed without diagnostics')
            for report in reports:
                key = (report['file'], report['line'], report['column'], report['code'], report['message'])
                findings[key] = report
    return [findings[key] for key in sorted(findings)]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=pathlib.Path, default=ROOT)
    parser.add_argument('--inventory', action='store_true', help='print tracked scope as JSON without running ShellCheck')
    parser.add_argument('--shellcheck', type=pathlib.Path, help='explicit binary; defaults to ShellCheck on PATH')
    args = parser.parse_args()
    try:
        root = args.root.resolve()
        inventory = tracked_inventory(root)
        if args.inventory:
            print(json.dumps(inventory, indent=2))
            return
        selected = args.shellcheck or shutil.which('shellcheck')
        provision.require(selected, 'ShellCheck missing; explicitly provision it and put it on PATH')
        binary = pathlib.Path(selected).resolve()
        artifact = provision.read_policy(ROOT)[2]['tools']['shellcheck']
        provision.check_binary(binary, artifact['platforms'][provision.host_platform()]['executable_sha256'],
                               artifact['version'], '--version')
        findings = scan(root, binary, inventory)
        for finding in findings:
            print(f'{json.dumps(finding["file"])}:{finding["line"]}:{finding["column"]}: '
                  f'{finding["level"]} SC{finding["code"]}: {finding["message"]}')
        print(f'shellcheck: {len(inventory["files"])} tracked scripts; {len(findings)} findings')
        if findings:
            sys.exit(1)
    except subprocess.CalledProcessError as error:
        detail = error.stderr.decode(errors='replace').strip() if isinstance(error.stderr, bytes) else error.stderr
        sys.exit(f'shellcheck: Git inventory failed: {detail or error}')
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        sys.exit(f'shellcheck: {error}')


if __name__ == '__main__':
    main()
