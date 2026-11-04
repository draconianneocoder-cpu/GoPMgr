# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Explicit ADR-013 auxiliary provisioning; Make owns all engineering tasks."""
import argparse
import hashlib
import json
import os
import pathlib
import platform
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import urllib.request

if sys.version_info < (3, 11):
    sys.exit('tools: Python 3.11 or newer is required (stdlib tomllib)')
import tomllib

ROOT = pathlib.Path(__file__).resolve().parents[2]


def read_policy(root):
    config_bytes = (root / 'mise.toml').read_bytes()
    lock_bytes = (root / 'mise.lock').read_bytes()
    config = tomllib.loads(config_bytes.decode())
    lock = tomllib.loads(lock_bytes.decode())
    inventory = json.loads((root / 'tools/engineering/artifacts.json').read_text())
    require(set(config) == {'tools', 'tool_config'}, 'only tools and tool_config are allowed')
    require(config['tool_config'] == {'locked': True}, 'locked tool configuration is required')
    require(set(inventory) == {'mise', 'tools'}, 'unexpected artifact inventory')
    expected_backends = {
        'shellcheck': 'github:koalaman/shellcheck', 'shfmt': 'github:mvdan/sh',
        'actionlint': 'github:rhysd/actionlint', 'zizmor': 'github:zizmorcore/zizmor',
    }
    require(set(inventory['tools']) == set(expected_backends), 'artifact tool inventory differs')
    backends = set(expected_backends.values())
    require(set(config['tools']) == backends, 'configured tool inventory differs')
    require(set(lock) == {'lockfile_version', 'tools'} and lock['lockfile_version'] == 3,
            'lock tool inventory differs')
    require(set(lock['tools']) == backends, 'lock tool inventory differs')
    for name, backend in expected_backends.items():
        tool = inventory['tools'][name]
        require(tool['backend'] == backend, 'artifact backend differs')
        require(re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', tool['version']), 'exact version required')
        require(set(tool['platforms']) == {'macos-arm64', 'linux-x64'}, 'artifact platform inventory differs')
        declaration = {'version': tool['version'], 'platforms': {}}
        if name == 'shfmt':
            declaration['rename_exe'] = 'shfmt'
        for platform, artifact in tool['platforms'].items():
            validate_artifact(artifact, backend.removeprefix('github:'), tool['version'])
            declaration['platforms'][platform] = {
                'asset_pattern': artifact['url'].rsplit('/', 1)[1],
                'checksum': 'sha256:' + artifact['archive_sha256'],
            }
        require(config['tools'][backend] == declaration, f'{name}: declaration differs from reviewed artifacts')
        entries = lock['tools'][backend]
        require(isinstance(entries, list) and len(entries) == 2, 'locked platform inventory differs')
        seen = set()
        for entry in entries:
            require(entry.get('version') == tool['version'] and entry.get('backend') == backend
                    and entry.get('specifiers') == [tool['version']], 'locked version differs')
            platform_keys = [key for key in entry if key.startswith('platforms.')]
            require(len(platform_keys) == 1, 'locked platform inventory differs')
            platform = platform_keys[0].removeprefix('platforms.')
            require(platform in tool['platforms'] and platform not in seen, 'locked platform inventory differs')
            seen.add(platform)
            artifact = tool['platforms'][platform]
            locked = entry[platform_keys[0]]
            require(locked.get('url') == artifact['url']
                    and locked.get('checksum') == 'sha256:' + artifact['archive_sha256'],
                    f'{name}: locked artifact differs')
            require(locked.get('url_api') == artifact['url_api'], 'unexpected artifact API')
            require(entry.get('options') == {'asset_pattern': artifact['url'].rsplit('/', 1)[1]},
                    'locked options differ')
            require(set(entry) == {'version', 'backend', 'specifiers', 'options', platform_keys[0]},
                    'unexpected locked entry')
            require(set(locked) <= {'url', 'url_api', 'checksum', 'checksum_source', 'provenance'},
                    'unexpected locked artifact fields')
    mise = inventory['mise']
    require(re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', mise['version']), 'exact mise version required')
    require(mise['release'] == f'https://github.com/jdx/mise/releases/tag/v{mise["version"]}',
            'unexpected mise release')
    require(set(mise['platforms']) == {'macos-arm64', 'linux-x64'}, 'mise platform inventory differs')
    for artifact in mise['platforms'].values():
        validate_artifact(artifact, 'jdx/mise', mise['version'])
    return config, lock, inventory, config_bytes, lock_bytes


def require(condition, message):
    if not condition:
        raise ValueError(message)


def validate_artifact(artifact, repo, version):
    require(set(artifact) == {'url', 'url_api', 'archive_sha256', 'member', 'executable_sha256'},
            'unexpected artifact metadata')
    prefix = f'https://github.com/{repo}/releases/download/v{version}/'
    require(artifact['url'].startswith(prefix) and re.fullmatch(r'[a-zA-Z0-9._-]+', artifact['url'][len(prefix):]),
            'unexpected release URL')
    require(re.fullmatch(r'https://api\.github\.com/repos/' + re.escape(repo)
                        + r'/releases/assets/[0-9]+', artifact['url_api']), 'unexpected artifact API')
    for key in ['archive_sha256', 'executable_sha256']:
        require(re.fullmatch(r'[0-9a-f]{64}', artifact[key]), 'invalid SHA256')
    member = pathlib.PurePosixPath(artifact['member'])
    require(not member.is_absolute() and '..' not in member.parts, 'invalid archive member')


def sha256(path):
    with path.open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def check_binary(path, digest, version, flag):
    require(sha256(path) == digest, f'{path}: executable checksum differs; refusing execution')
    result = subprocess.run([str(path.resolve()), flag], capture_output=True, text=True,
                            timeout=15, env={'PATH': '/usr/bin:/bin', 'LANG': 'C'})
    require(result.returncode == 0 and re.search(r'(?<![0-9.])v?' + re.escape(version)
                                               + r'(?![0-9.])', result.stdout),
            f'{path}: expected version {version}, got {result.stdout.strip()!r}')


def publish_archive(archive, artifact, destination):
    require(sha256(archive) == artifact['archive_sha256'], 'archive checksum differs')
    with tarfile.open(archive) as source:
        matches = [member for member in source.getmembers() if member.name == artifact['member']]
        require(len(matches) == 1 and matches[0].isfile() and matches[0].size <= 256 * 1024 * 1024,
                'expected one bounded regular archive member')
        data = source.extractfile(matches[0]).read()
    require(hashlib.sha256(data).hexdigest() == artifact['executable_sha256'], 'executable checksum differs')
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(dir=destination.parent, delete=False) as output:
        temporary = pathlib.Path(output.name)
        try:
            output.write(data)
            output.flush()
            os.fchmod(output.fileno(), 0o755)
            os.replace(temporary, destination)
        finally:
            temporary.unlink(missing_ok=True)


VERSION_FLAGS = {'shellcheck': '--version', 'shfmt': '-version',
                 'actionlint': '-version', 'zizmor': '--version'}


def host_platform():
    pair = (platform.system(), platform.machine())
    supported = {('Darwin', 'arm64'): 'macos-arm64', ('Linux', 'x86_64'): 'linux-x64'}
    require(pair in supported, f'P1 provisioning is not qualified for host {pair}')
    return supported[pair]


def bootstrap(state, artifact, archive):
    if archive is not None:
        publish_archive(archive, artifact, state / 'mise')
        return
    state.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=state, prefix='download-') as directory:
        download = pathlib.Path(directory) / 'mise.tar.xz'
        with urllib.request.urlopen(artifact['url'], timeout=30) as source, download.open('wb') as output:
            total = 0
            while chunk := source.read(1024 * 1024):
                total += len(chunk)
                require(total <= 256 * 1024 * 1024, 'bootstrap archive exceeds size limit')
                output.write(chunk)
        publish_archive(download, artifact, state / 'mise')


def mise_environment(state, stage, backends, safe):
    # Never pass caller tokens, shell hooks, proxy settings or mise overrides.
    return {
        'PATH': '/usr/bin:/bin', 'LANG': 'C', 'HOME': str(state / 'home'),
        'XDG_CONFIG_HOME': str(state / 'config'), 'XDG_DATA_HOME': str(state / 'data'),
        'XDG_CACHE_HOME': str(state / 'cache'), 'XDG_STATE_HOME': str(state / 'state'),
        'MISE_CONFIG_DIR': str(state / 'config/mise'),
        'MISE_DATA_DIR': str(state / 'data/mise'), 'MISE_CACHE_DIR': str(state / 'cache/mise'),
        'MISE_GLOBAL_CONFIG_FILE': str(stage / 'empty.toml'),
        'MISE_SYSTEM_CONFIG_FILE': str(stage / 'empty.toml'),
        'MISE_CEILING_PATHS': str(stage.parent), 'MISE_OVERRIDE_CONFIG_FILENAMES': 'mise.toml',
        'MISE_ENABLE_TOOLS': ','.join(backends), 'MISE_IDIOMATIC_VERSION_FILE_ENABLE_TOOLS': '',
        'MISE_NO_ENV': '1', 'MISE_NO_HOOKS': '1', 'MISE_SKILLS_AUTO_SYNC': '0',
        'MISE_SAFE': '1' if safe else '0', 'MISE_COLOR': '0', 'MISE_PARANOID': '1',
    }


def check_tools(inventory, platform_name):
    for name, tool in inventory['tools'].items():
        path = shutil.which(name)
        require(path is not None, f'{name}: missing on PATH; explicitly install approved tools first')
        check_binary(pathlib.Path(path), tool['platforms'][platform_name]['executable_sha256'],
                     tool['version'], VERSION_FLAGS[name])
        print(f'tools: {name} {tool["version"]} verified: {path}')
        selected = pathlib.Path(path).resolve()
        shadowed = set()
        for directory in os.get_exec_path():
            candidate = pathlib.Path(directory) / name
            if candidate.is_file() and os.access(candidate, os.X_OK) and candidate.resolve() != selected:
                shadowed.add(str(candidate))
        for candidate in sorted(shadowed):
            print(f'tools: shadowed {name} (not executed): {candidate}')
    for dependency in ['git', 'make', 'go', 'node', 'npm', 'cc']:
        require(shutil.which(dependency) is not None, f'missing host dependency: {dependency}')
    if platform_name == 'linux-x64':
        pkgconfig = pathlib.Path('/usr/bin/pkg-config')
        require(pkgconfig.is_file(), 'missing host dependency: /usr/bin/pkg-config')
        result = subprocess.run([str(pkgconfig), '--exists', 'gtk4', 'webkitgtk-6.0'], timeout=15,
                                env={'PATH': '/usr/bin:/bin', 'LANG': 'C'})
        require(result.returncode == 0, 'GTK 4 and WebKitGTK 6.0 development packages are required')
    print('tools: host command presence verified; canonical version/native gates remain separate')


def check_config_sources(mise, stage, environment):
    result = subprocess.run([str(mise), 'config', 'ls', '--json'], cwd=stage,
                            env=environment, capture_output=True, text=True, check=True, timeout=30)
    sources = json.loads(result.stdout)
    expected = {stage / 'mise.toml', stage / 'empty.toml'}
    require(isinstance(sources, list)
            and {pathlib.Path(source['path']).resolve() for source in sources} == expected,
            'unexpected configuration sources; refusing install')


def install(state, policy, platform_name, safe):
    state = state.resolve()
    config, _, inventory, config_bytes, lock_bytes = policy
    mise = state / 'mise'
    require(mise.is_file(), 'mise missing; run make tools-bootstrap explicitly')
    check_binary(mise, inventory['mise']['platforms'][platform_name]['executable_sha256'],
                 inventory['mise']['version'], '--version')
    # The immutable copy is the only project configuration mise may consume.
    # Do not trust the working tree, user config, ancestors, or local overrides.
    with tempfile.TemporaryDirectory(dir=state, prefix='provision-') as directory:
        stage = pathlib.Path(directory)
        (stage / 'mise.toml').write_bytes(config_bytes)
        (stage / 'mise.lock').write_bytes(lock_bytes)
        (stage / 'empty.toml').write_text('[settings]\ncolor = false\n')
        environment = mise_environment(state, stage, config['tools'], safe)
        # Normal-mode global-config trust canonicalizes HOME before installing.
        (state / 'home').mkdir(exist_ok=True)
        subprocess.run([str(mise), 'trust', 'mise.toml'], cwd=stage, env=environment, check=True, timeout=30)
        check_config_sources(mise, stage, environment)
        subprocess.run([str(mise), 'install', '--locked'], cwd=stage, env=environment, check=True, timeout=300)
    verified = {}
    for name, tool in inventory['tools'].items():
        directory = tool['backend'].replace(':', '-').replace('/', '-')
        path = state / 'data/mise/installs' / directory / tool['version'] / name
        require(path.resolve().is_relative_to(state), 'installed binary escaped task state')
        check_binary(path, tool['platforms'][platform_name]['executable_sha256'], tool['version'], VERSION_FLAGS[name])
        verified[name] = path
    destination = state / 'bin'
    destination.mkdir(exist_ok=True)
    for name, path in verified.items():
        temporary = destination / (name + '.new')
        temporary.unlink(missing_ok=True)
        temporary.symlink_to(path)
        temporary.replace(destination / name)
    print(f'tools: verified tools available at {destination}; PATH is unchanged')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=['policy', 'check', 'bootstrap', 'install'])
    parser.add_argument('--root', type=pathlib.Path, default=ROOT)
    parser.add_argument('--state', type=pathlib.Path, help='task-only state; default .tmp/engineering')
    parser.add_argument('--archive', type=pathlib.Path, help='bootstrap from an already downloaded pinned archive')
    parser.add_argument('--safe', action='store_true', help='qualify install with MISE_SAFE=1')
    args = parser.parse_args()
    try:
        policy = read_policy(args.root)
        if args.command == 'policy':
            print('tools: policy verified')
            return
        platform_name = host_platform()
        state = (args.state or args.root / '.tmp/engineering').resolve()
        if args.command == 'check':
            check_tools(policy[2], platform_name)
        elif args.command == 'bootstrap':
            bootstrap(state, policy[2]['mise']['platforms'][platform_name], args.archive)
        else:
            install(state, policy, platform_name, args.safe)
    except (OSError, ValueError, KeyError, TypeError, tarfile.TarError, subprocess.SubprocessError) as error:
        sys.exit(f'tools: {error}')


if __name__ == '__main__':
    main()
