# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Regression checks for the explicit auxiliary-tool provisioning boundary."""
import pathlib
import importlib.util
import hashlib
import json
import io
import os
from unittest import mock
import tarfile
import shutil
import tempfile
import subprocess
import sys
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
HELPER = ROOT / 'scripts/engineering/provision.py'


class PolicyTests(unittest.TestCase):
    def test_accepts_coherent_approved_policy_without_mise(self):
        result = subprocess.run([sys.executable, '-B', str(HELPER), 'policy'],
                                cwd=ROOT, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('policy verified', result.stdout)


class RejectedPolicyTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix='provision policy ')
        self.addCleanup(self.directory.cleanup)
        self.root = pathlib.Path(self.directory.name)
        (self.root / 'tools/engineering').mkdir(parents=True)
        for name in ['mise.toml', 'mise.lock', 'tools/engineering/artifacts.json']:
            shutil.copyfile(ROOT / name, self.root / name)

    def reject(self, filename, mutate, diagnostic):
        path = self.root / filename
        path.write_text(mutate(path.read_text()))
        result = subprocess.run([sys.executable, '-B', str(HELPER), 'policy',
                                 '--root', str(self.root)], capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0, result.stdout)
        self.assertIn(diagnostic, result.stderr)

    def test_rejects_code_bearing_config_before_trust(self):
        for section in ['tasks', 'hooks', 'env', 'plugins', 'settings', 'bootstrap']:
            with self.subTest(section=section):
                original = (ROOT / 'mise.toml').read_text()
                (self.root / 'mise.toml').write_text(original)
                self.reject('mise.toml', lambda s: s + f'\n[{section}]\ncommand = "touch sentinel"\n',
                            'only tools and tool_config')

    def test_rejects_version_drift(self):
        self.reject('mise.toml', lambda s: s.replace('3.14.1', '3.14.2'), 'declaration differs')

    def test_rejects_lock_checksum_substitution(self):
        self.reject('mise.lock', lambda s: s.replace('sha256:8c3be12b', 'sha256:00000000'),
                    'locked artifact differs')

    def test_rejects_missing_platform_checksum(self):
        self.reject('mise.lock', lambda s: '\n'.join(line for line in s.splitlines()
                    if not line.startswith('checksum =')), 'locked artifact differs')

    def test_rejects_truncated_lock(self):
        self.reject('mise.lock', lambda s: s[:s.index('[[tools.')], 'lock tool inventory differs')

    def test_rejects_unreviewed_backend(self):
        self.reject('mise.toml', lambda s: s.replace('github:mvdan/sh', 'cargo:shfmt'),
                    'configured tool inventory differs')

    def test_rejects_release_url_substitution(self):
        self.reject('mise.lock', lambda s: s.replace('https://github.com/', 'https://untrusted.invalid/'),
                    'locked artifact differs')

    def test_rejects_api_url_substitution(self):
        self.reject('mise.lock', lambda s: s.replace('https://api.github.com/', 'https://untrusted.invalid/'),
                    'unexpected artifact API')

    def test_rejects_different_api_asset_in_the_same_repository(self):
        self.reject('mise.lock', lambda s: s.replace('/assets/279056942', '/assets/279056943'),
                    'unexpected artifact API')

    def test_rejects_renamed_executable_option(self):
        self.reject('mise.toml', lambda s: s.replace('rename_exe = "shfmt"', 'rename_exe = "go"'),
                    'declaration differs')

    def test_safe_install_rejects_substituted_url_before_mise_or_state_creation(self):
        lock = self.root / 'mise.lock'
        lock.write_text(lock.read_text().replace('https://github.com/', 'https://127.0.0.1:9/'))
        state = self.root / 'state'
        result = subprocess.run([sys.executable, '-B', str(HELPER), 'install', '--safe',
                                 '--root', str(self.root), '--state', str(state)],
                                capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('locked artifact differs', result.stderr)
        self.assertFalse(state.exists())

    def test_safe_install_rejects_url_before_any_mise_subprocess(self):
        state = self.root / 'state'
        state.mkdir()
        marker = self.root / 'mise-executed'
        binary = state / 'mise'
        binary.write_text(f'#!/bin/sh\n/usr/bin/touch "{marker}"\necho "2026.11.0"\n')
        binary.chmod(0o755)
        path = self.root / 'tools/engineering/artifacts.json'
        inventory = json.loads(path.read_text())
        for artifact in inventory['mise']['platforms'].values():
            artifact['executable_sha256'] = hashlib.sha256(binary.read_bytes()).hexdigest()
        path.write_text(json.dumps(inventory))
        lock = self.root / 'mise.lock'
        lock.write_text(lock.read_text().replace('https://github.com/', 'https://127.0.0.1:9/'))
        result = subprocess.run([sys.executable, '-B', str(HELPER), 'install', '--safe',
                                 '--root', str(self.root), '--state', str(state)],
                                capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('locked artifact differs', result.stderr)
        self.assertFalse(marker.exists(), 'mise ran before URL validation')

    def test_install_refuses_unsafe_config_before_creating_state(self):
        config = self.root / 'mise.toml'
        config.write_text(config.read_text() + '\n[settings.skills]\nauto_sync = true\n')
        state = self.root / 'state'
        result = subprocess.run([sys.executable, '-B', str(HELPER), 'install', '--root', str(self.root),
                                 '--state', str(state)], capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('only tools and tool_config', result.stderr)
        self.assertFalse(state.exists())


class ArtifactTests(unittest.TestCase):
    def setUp(self):
        spec = importlib.util.spec_from_file_location('provision', HELPER)
        self.helper = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(self.helper)
        self.directory = tempfile.TemporaryDirectory(prefix='provision binary ')
        self.addCleanup(self.directory.cleanup)
        self.root = pathlib.Path(self.directory.name)

    def test_rejects_fake_version_without_executing_it(self):
        sentinel = self.root / 'executed'
        binary = self.root / 'shellcheck'
        binary.write_text(f'#!/bin/sh\ntouch "{sentinel}"\necho "Version: 0.11.0"\n')
        binary.chmod(0o755)
        with self.assertRaisesRegex(ValueError, 'executable checksum'):
            self.helper.check_binary(binary, '0' * 64, '0.11.0', '--version')
        self.assertFalse(sentinel.exists())

    def test_detects_wrong_version_even_with_reviewed_bytes(self):
        binary = self.root / 'tool'
        binary.write_text('#!/bin/sh\necho "Version: 0.10.0"\n')
        binary.chmod(0o755)
        digest = hashlib.sha256(binary.read_bytes()).hexdigest()
        with self.assertRaisesRegex(ValueError, 'expected version'):
            self.helper.check_binary(binary, digest, '0.11.0', '--version')

    def test_accepts_verified_binary_from_path_with_spaces(self):
        binary = self.root / 'tool'
        binary.write_text('#!/bin/sh\necho "Version: 0.11.0"\n')
        binary.chmod(0o755)
        digest = hashlib.sha256(binary.read_bytes()).hexdigest()
        self.helper.check_binary(binary, digest, '0.11.0', '--version')

    def archive(self, member='mise/bin/mise', data=b'new binary', symlink=False):
        archive = self.root / 'archive.tar.xz'
        with tarfile.open(archive, 'w:xz') as output:
            info = tarfile.TarInfo(member)
            info.size = len(data)
            if symlink:
                info.type = tarfile.SYMTYPE
                info.linkname = '/tmp/unselected'
            output.addfile(info, io.BytesIO(data))
        return archive, {
            'archive_sha256': hashlib.sha256(archive.read_bytes()).hexdigest(),
            'member': 'mise/bin/mise',
            'executable_sha256': hashlib.sha256(data).hexdigest(),
        }

    def test_bootstrap_publishes_only_verified_selected_member(self):
        archive, artifact = self.archive()
        destination = self.root / 'bin/mise'
        self.helper.publish_archive(archive, artifact, destination)
        self.assertEqual(destination.read_bytes(), b'new binary')
        self.assertTrue(destination.stat().st_mode & 0o111)

    def test_bootstrap_checksum_failure_preserves_existing_binary(self):
        archive, artifact = self.archive()
        destination = self.root / 'mise'
        destination.write_bytes(b'previous verified binary')
        artifact['archive_sha256'] = '0' * 64
        with self.assertRaisesRegex(ValueError, 'archive checksum'):
            self.helper.publish_archive(archive, artifact, destination)
        self.assertEqual(destination.read_bytes(), b'previous verified binary')

    def test_bootstrap_refuses_symlink_member(self):
        archive, artifact = self.archive(symlink=True)
        destination = self.root / 'mise'
        with self.assertRaisesRegex(ValueError, 'regular archive member'):
            self.helper.publish_archive(archive, artifact, destination)
        self.assertFalse(destination.exists())

    def test_bootstrap_rejects_extracted_binary_corruption(self):
        archive, artifact = self.archive()
        artifact['executable_sha256'] = '0' * 64
        destination = self.root / 'mise'
        destination.write_bytes(b'previous verified binary')
        with self.assertRaisesRegex(ValueError, 'executable checksum'):
            self.helper.publish_archive(archive, artifact, destination)
        self.assertEqual(destination.read_bytes(), b'previous verified binary')


class IsolationTests(unittest.TestCase):
    setUp = ArtifactTests.setUp
    archive = ArtifactTests.archive

    def test_isolated_mise_environment_drops_credentials_and_overrides(self):
        with mock.patch.dict(os.environ, {'GITHUB_TOKEN': 'private', 'MISE_SKILLS_AUTO_SYNC': '1',
                                         'MISE_CONFIG_DIR': '/untrusted', 'PATH': '/untrusted'}):
            env = self.helper.mise_environment(self.root, self.root / 'stage', ['github:mvdan/sh'], False)
        self.assertNotIn('GITHUB_TOKEN', env)
        self.assertEqual(env['PATH'], '/usr/bin:/bin')
        self.assertEqual(env['MISE_SKILLS_AUTO_SYNC'], '0')
        self.assertEqual(env.get('MISE_PARANOID'), '1')
        self.assertEqual(env['MISE_IDIOMATIC_VERSION_FILE_ENABLE_TOOLS'], '')
        self.assertEqual(env['MISE_ENABLE_TOOLS'], 'github:mvdan/sh')
        self.assertTrue(env['HOME'].startswith(str(self.root)))

    def test_bootstrap_uses_supplied_verified_archive_without_network(self):
        archive, artifact = self.archive()
        with mock.patch('urllib.request.urlopen', side_effect=AssertionError('unexpected network')):
            self.helper.bootstrap(self.root, artifact, archive)
        self.assertEqual((self.root / 'mise').read_bytes(), b'new binary')

    def test_download_failure_does_not_publish_mise(self):
        _, artifact = self.archive()
        artifact['url'] = 'https://unreachable.invalid/mise.tar.xz'
        with mock.patch('urllib.request.urlopen', side_effect=OSError('network unavailable')):
            with self.assertRaisesRegex(OSError, 'network unavailable'):
                self.helper.bootstrap(self.root, artifact, None)
        self.assertFalse((self.root / 'mise').exists())

    def test_read_only_check_rejects_wrong_path_candidate_without_execution(self):
        binary = self.root / 'shellcheck'
        sentinel = self.root / 'executed'
        binary.write_text(f'#!/bin/sh\ntouch "{sentinel}"\necho 0.11.0\n')
        binary.chmod(0o755)
        inventory = {'tools': {'shellcheck': {'version': '0.11.0', 'platforms': {
            'macos-arm64': {'executable_sha256': '0' * 64}}}}}
        with mock.patch.dict(os.environ, {'PATH': str(self.root)}):
            with self.assertRaisesRegex(ValueError, 'executable checksum'):
                self.helper.check_tools(inventory, 'macos-arm64')
        self.assertFalse(sentinel.exists())

    def test_linux_pkg_config_shadow_is_never_executed(self):
        marker = self.root / 'pkg-config-executed'
        for name in ['git', 'make', 'go', 'node', 'npm', 'cc', 'pkg-config']:
            binary = self.root / name
            binary.write_text(f'#!/bin/sh\n/usr/bin/touch "{marker}"\n')
            binary.chmod(0o755)
        with mock.patch.dict(os.environ, {'PATH': str(self.root)}):
            try:
                self.helper.check_tools({'tools': {}}, 'linux-x64')
            except ValueError as error:
                self.assertRegex(str(error), 'host dependency|GTK 4')
        self.assertFalse(marker.exists(), 'unverified PATH pkg-config executed')


if __name__ == '__main__':
    unittest.main()
