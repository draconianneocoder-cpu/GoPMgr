# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Opt-in network qualification against the pinned, already bootstrapped mise."""
import importlib.util
import os
import pathlib
import shutil
import tempfile
import subprocess
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('provision', ROOT / 'scripts/engineering/provision.py')
provision = importlib.util.module_from_spec(spec)
spec.loader.exec_module(provision)


def snapshot(root, excluded):
    result = {}
    for path in root.rglob('*'):
        if path.is_relative_to(excluded):
            continue
        value = ('link', os.readlink(path)) if path.is_symlink() else (
            ('directory',) if path.is_dir() else ('file', path.read_bytes()))
        result[str(path.relative_to(root))] = value
    return result


class LockedInstallTests(unittest.TestCase):
    def test_trusted_and_safe_install_ignore_hostile_ambient_configuration(self):
        source_mise = ROOT / '.tmp/engineering/mise'
        self.assertTrue(source_mise.is_file(), 'run make tools-bootstrap before network qualification')
        for safe in [False, True]:
            with self.subTest(safe=safe), tempfile.TemporaryDirectory(prefix='p1 qualify ') as directory:
                root = pathlib.Path(directory)
                state = root / 'state'
                state.mkdir()
                shutil.copyfile(source_mise, state / 'mise')
                (state / 'mise').chmod(0o755)
                sentinel = root / 'executed'
                hostile = f"[hooks]\nenter = \"touch '{sentinel}'\"\n"
                (root / 'mise.toml').write_text(hostile)
                (root / 'mise.local.toml').write_text(hostile)
                (root / 'go.mod').write_text('module fixture\n\ngo 1.27.3\n')
                (root / '.tool-versions').write_text('go 1.27.3\nnode 26.8.2\n')
                for skills in ['.agents/skills', '.claude/skills']:
                    target = root / skills / 'fixture'
                    target.mkdir(parents=True)
                    (target / 'SKILL.md').write_text('unchanged')
                before = snapshot(root, state)
                old = os.environ.get('MISE_SKILLS_AUTO_SYNC')
                os.environ['MISE_SKILLS_AUTO_SYNC'] = '1'
                try:
                    provision.install(state, provision.read_policy(ROOT), provision.host_platform(), safe)
                finally:
                    if old is None:
                        os.environ.pop('MISE_SKILLS_AUTO_SYNC', None)
                    else:
                        os.environ['MISE_SKILLS_AUTO_SYNC'] = old
                self.assertFalse(sentinel.exists())
                after = snapshot(root, state)
                self.assertEqual(after, before)
                self.assertEqual({p.name for p in (state / 'bin').iterdir()},
                                 {'shellcheck', 'shfmt', 'actionlint', 'zizmor'})
                self.assertFalse((state / 'data/mise/installs/go').exists())
                if not safe:
                    cached = state / 'data/mise/installs/github-koalaman-shellcheck/0.11.0/shellcheck'
                    cached.write_text(f'#!/bin/sh\n/usr/bin/touch "{sentinel}"\necho "Version: 0.11.0"\n')
                    cached.chmod(0o755)
                    with self.assertRaisesRegex(ValueError, 'executable checksum'):
                        provision.install(state, provision.read_policy(ROOT), provision.host_platform(), False)
                    self.assertFalse(sentinel.exists())


    def test_sources_exclude_ancestor_local_and_user_configuration(self):
        with tempfile.TemporaryDirectory(prefix='p1 sources ') as directory:
            root = pathlib.Path(directory).resolve()
            stage = root / 'stage'
            state = root / 'state'
            stage.mkdir()
            (state / 'home/.config/mise').mkdir(parents=True)
            (root / 'mise.toml').write_text('[tools]\ngo = "1.27.3"\n')
            (stage / 'mise.local.toml').write_text('[tools]\nnode = "26.8.2"\n')
            (state / 'home/.config/mise/config.toml').write_text('[settings.skills]\nauto_sync = true\n')
            for name in ['mise.toml', 'mise.lock']:
                shutil.copyfile(ROOT / name, stage / name)
            (stage / 'empty.toml').write_text('[settings]\ncolor = false\n')
            env = provision.mise_environment(state, stage, provision.read_policy(ROOT)[0]['tools'], False)
            mise = (ROOT / '.tmp/engineering/mise').resolve()
            subprocess.run([str(mise), 'trust', 'mise.toml'], cwd=stage, env=env, check=True, timeout=30)
            provision.check_config_sources(mise, stage, env)
            del env['MISE_CEILING_PATHS']
            with self.assertRaises((ValueError, subprocess.CalledProcessError)):
                provision.check_config_sources(mise, stage, env)

    def test_real_locked_install_refuses_invalid_locks_and_untrusted_config(self):
        for failure in ['missing-lock', 'missing-checksum', 'wrong-checksum', 'untrusted', 'no-network']:
            with self.subTest(failure=failure), tempfile.TemporaryDirectory(prefix='p1 negative ') as directory:
                stage = pathlib.Path(directory).resolve()
                state = stage / 'state'
                (state / 'home').mkdir(parents=True)
                (stage / 'empty.toml').write_text('[settings]\ncolor = false\n')
                shutil.copyfile(ROOT / 'mise.toml', stage / 'mise.toml')
                lock = (ROOT / 'mise.lock').read_text()
                if failure == 'missing-checksum':
                    lock = '\n'.join(line for line in lock.splitlines() if not line.startswith('checksum ='))
                if failure == 'wrong-checksum':
                    import re
                    lock = re.sub(r'sha256:[0-9a-f]{64}', 'sha256:' + '0' * 64, lock)
                if failure != 'missing-lock':
                    (stage / 'mise.lock').write_text(lock)
                env = provision.mise_environment(state, stage, ['github:koalaman/shellcheck'], False)
                mise = str((ROOT / '.tmp/engineering/mise').resolve())
                if failure != 'untrusted':
                    subprocess.run([mise, 'trust', 'mise.toml'], cwd=stage, env=env, check=True, timeout=30,
                                   capture_output=True)
                if failure == 'no-network':
                    env['MISE_OFFLINE'] = '1'
                result = subprocess.run([mise, 'install', '--locked', 'github:koalaman/shellcheck@0.11.0'],
                                        cwd=stage, env=env, capture_output=True, text=True, timeout=90)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                diagnostic = (result.stdout + result.stderr).lower()
                expected = {'missing-lock': 'lock', 'missing-checksum': 'checksum',
                            'wrong-checksum': 'checksum', 'untrusted': 'trust', 'no-network': 'offline'}
                self.assertIn(expected[failure], diagnostic)
                self.assertFalse((state / 'data/mise/installs/github-koalaman-shellcheck/0.11.0/shellcheck').exists())
                print(f'negative {failure}: exit {result.returncode}; expected diagnostic verified')



if __name__ == '__main__':
    unittest.main()
