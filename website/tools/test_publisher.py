# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Publisher trust-boundary cases; no live publication or credentials."""
import copy
from datetime import datetime, timedelta, timezone
import unittest
import publisher


class GrantTests(unittest.TestCase):
    def test_artifact_specific_owner_grant_rejects_untrusted_inputs(self):
        binding = dict(commit='1'*40, tree='2'*40, expected_tip=None,
                       source_digest='3'*64, output_digest='4'*64,
                       acceptance_sha256='5'*64)
        repo = dict(id=1277267642, full_name='draconianneocoder-cpu/GoPMgr',
                    owner={'id': 284033065}, permissions={'push': True})
        actor = {'id': 284033065}
        grant = dict(schema_version=1, approval_id='owner-fixture-1', status='approved',
                     expires_at=(datetime.now(timezone.utc)+timedelta(hours=1)).isoformat(),
                     repository='draconianneocoder-cpu/GoPMgr', repository_id=1277267642,
                     ref='refs/heads/gh-pages', approved_by=284033065,
                     publisher_id=284033065, **binding)
        publisher.validate_grant(grant, binding, actor, repo)
        cases = [
            ('repository', 'someone/else', 'destination'),
            ('repository_id', 1, 'destination'),
            ('ref', 'refs/heads/main', 'destination'),
            ('publisher_id', 1, 'publisher'),
            ('approved_by', 1, 'owner'),
            ('status', 'revoked', 'approved'),
            ('expires_at', '2000-01-01T00:00:00+00:00', 'expired'),
            ('expires_at', '2099-01-01T00:00:00', 'timezone'),
            ('acceptance_sha256', '6'*64, 'acceptance_sha256'),
            ('commit', '6'*40, 'commit'),
            ('expected_tip', '6'*40, 'expected_tip'),
        ]
        for key, value, reason in cases:
            with self.subTest(key=key):
                bad = copy.deepcopy(grant)
                bad[key] = value
                with self.assertRaisesRegex(ValueError, reason):
                    publisher.validate_grant(bad, binding, actor, repo)
        for bad_actor, bad_repo in [({'id': 1}, repo), (actor, {**repo, 'id': 1}),
                                    (actor, {**repo, 'permissions': {'push': False}})]:
            with self.subTest(actor=bad_actor, repo=bad_repo):
                with self.assertRaises(ValueError):
                    publisher.validate_grant(grant, binding, bad_actor, bad_repo)


class CandidateTests(unittest.TestCase):
    def test_raw_tree_rejects_unlisted_payload_and_preserves_immutable_source(self):
        from pathlib import Path
        import tempfile
        import publication
        import test_publication
        import subprocess
        from unittest.mock import patch
        with tempfile.TemporaryDirectory() as tmp:
            public = Path(tmp) / 'public'
            public.mkdir()
            (public / '.nojekyll').touch()
            (public / 'index.html').write_text('Accepted bytes')
            expected = test_publication.site.inventory(public)
            candidate = publication.stage(public, expected, test_publication.site.inventory)
            repo = Path(candidate['repository'])
            # Candidate-local rewriting cannot affect a fresh transport repo.
            remote = Path(tmp) / 'remote.git'
            publication.git(Path(tmp), 'init', '--quiet', '--bare', str(remote))
            publication.git(repo, 'config', 'url.' + str(Path(tmp)/'nonexistent') + '.insteadOf', str(remote))
            (public / 'extra.txt').write_text('not accepted')
            changed = publication.stage(public, test_publication.site.inventory(public), test_publication.site.inventory)
            publication.git(repo, '-c', 'protocol.file.allow=always', 'fetch', '--quiet', changed['repository'], changed['commit'])
            publication.git(repo, 'update-ref', 'refs/heads/gh-pages', changed['commit'])
            with publisher.transport_repository(repo, candidate['commit']) as transport:
                publisher.verify_tree(transport, candidate['commit'], expected)
                self.assertEqual(publication.git(transport, 'rev-parse', candidate['commit'] + '^{tree}'), candidate['tree'])
                original = subprocess.run
                def local_transport(args, **kwargs):
                    return original([str(remote) if arg == publisher.DESTINATION else arg for arg in args], **kwargs)
                with patch('publisher.subprocess.run', side_effect=local_transport):
                    publisher.transfer(transport, candidate['commit'], None, 'fixture-secret')
                self.assertEqual(publication.git(remote, 'rev-parse', 'refs/heads/gh-pages'), candidate['commit'])
            with self.assertRaisesRegex(ValueError, 'unexpected'):
                publisher.verify_tree(Path(changed['repository']), changed['commit'], expected)

    def test_final_revocation_prevents_any_push(self):
        import hashlib
        import json
        from pathlib import Path
        import tempfile
        from unittest.mock import patch
        import publication
        import test_publication
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            public = root / 'public'
            public.mkdir()
            (public / '.nojekyll').touch()
            (public / 'index.html').write_text('Accepted bytes')
            output = test_publication.site.inventory(public)
            candidate = publication.stage(public, output, test_publication.site.inventory)
            report = {'source': {'digest': '3'*64}, 'output': output, 'output_digest': '4'*64}
            acceptance = dict(schema_version=1, reviewer='fixture', reviewed_at='2026-11-15',
                              source_digest='3'*64, output_digest='4'*64,
                              private_channel='https://github.com/draconianneocoder-cpu/GoPMgr/security/advisories/new',
                              owner_channel_approval=True,
                              gates={**{f'A{i:02}':'PASS' for i in range(1,16)}, **{f'R{i}':'PASS' for i in range(1,12)}})
            accept_path = root / 'acceptance.json'
            accept_path.write_text(json.dumps(acceptance))
            grant = dict(schema_version=1, approval_id='fixture', status='approved',
                         expires_at=(datetime.now(timezone.utc)+timedelta(hours=1)).isoformat(),
                         repository='draconianneocoder-cpu/GoPMgr', repository_id=1277267642,
                         ref='refs/heads/gh-pages', approved_by=284033065, publisher_id=284033065,
                         commit=candidate['commit'], tree=candidate['tree'], expected_tip=None,
                         source_digest='3'*64, output_digest='4'*64,
                         acceptance_sha256=hashlib.sha256(accept_path.read_bytes()).hexdigest())
            grant_path = root / 'grant.json'
            grant_path.write_text(json.dumps(grant))
            # Supply authenticated facts, not the authorization being asserted.
            repo = {'id':1277267642, 'full_name':'draconianneocoder-cpu/GoPMgr',
                    'owner':{'id':284033065}, 'permissions':{'push':True}}
            queries = []
            def observe_then_revoke(*args):
                queries.append(None)
                if len(queries) == 2:
                    grant_path.write_text(json.dumps({**grant, 'status':'revoked'}))
                return None
            with patch.object(publisher.site, 'check_report', return_value=report), \
                    patch('publisher.credential', return_value='fixture-secret'), \
                    patch('publisher.github_identity', return_value=({'id':284033065}, repo)), \
                    patch('publisher.remote_tip', return_value=None) as observe, \
                    patch('publisher.transfer') as push:
                preflight = publisher.transaction(root/'report.json', accept_path, Path(candidate['repository']), grant_path)
                self.assertEqual((preflight['authorized_preflight'], preflight['published']), (True, False))
                push.assert_not_called()
                observe.side_effect = observe_then_revoke
                with self.assertRaisesRegex(ValueError, 'grant changed'):
                    publisher.transaction(root/'report.json', accept_path, Path(candidate['repository']), grant_path, execute=True)
                push.assert_not_called()
                self.assertEqual(len(queries), 2)


    def test_shallow_metadata_cannot_hide_a_real_parent(self):
        from pathlib import Path
        import tempfile
        import publication
        import test_publication
        with tempfile.TemporaryDirectory() as tmp:
            public = Path(tmp) / 'public'
            public.mkdir()
            (public / '.nojekyll').touch()
            first = publication.stage(public, test_publication.site.inventory(public), test_publication.site.inventory)
            second = publication.stage(public, test_publication.site.inventory(public), test_publication.site.inventory,
                                       Path(first['repository']), first['commit'])
            repo = Path(second['repository'])
            (repo/'.git/shallow').write_text(second['commit']+'\n')
            with self.assertRaisesRegex(ValueError, 'sole parent'):
                publisher.verify_parent(repo, second['commit'], None)

    def test_malformed_candidate_has_only_sanitized_cli_failure(self):
        import contextlib
        import io
        from pathlib import Path
        import tempfile
        from unittest.mock import patch
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            bad = root / 'private-sentinel'
            (bad/'.git').mkdir(parents=True)
            (bad/'.git/config').write_text('[invalid')
            stderr = io.StringIO()
            with patch('sys.argv', ['publisher', '--report', str(root/'report'),
                                   '--acceptance', str(root/'acceptance'),
                                   '--candidate', str(bad), '--grant', str(root/'grant')]), \
                    patch.object(publisher.site, 'check_report', return_value={'source':{}}), \
                    contextlib.redirect_stderr(stderr):
                self.assertEqual(publisher.cli(), 1)
            self.assertEqual(stderr.getvalue(), 'website publisher: refused or unconfirmed; review local approval and remote state\n')


    def test_nonobject_private_records_refuse_without_credentials_or_transport(self):
        import contextlib
        import io
        import json
        from pathlib import Path
        import tempfile
        from unittest.mock import patch
        import publication
        import test_publication
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            public = root/'public'
            public.mkdir()
            (public/'.nojekyll').touch()
            output = test_publication.site.inventory(public)
            candidate = publication.stage(public, output, test_publication.site.inventory)
            report = {'source':{'digest':'3'*64}, 'output':output, 'output_digest':'4'*64}
            acceptance = dict(schema_version=1, reviewer='fixture', reviewed_at='2026-11-15',
                              source_digest='3'*64, output_digest='4'*64,
                              private_channel='https://github.com/draconianneocoder-cpu/GoPMgr/security/advisories/new',
                              owner_channel_approval=True,
                              gates={**{f'A{i:02}':'PASS' for i in range(1,16)}, **{f'R{i}':'PASS' for i in range(1,12)}})
            for field in ('acceptance', 'grant'):
                for shape in ([], None):
                    with self.subTest(field=field, shape=shape):
                        (root/'acceptance').write_text(json.dumps(acceptance))
                        (root/'grant').write_text('{}')
                        (root/field).write_text(json.dumps(shape))
                        stderr = io.StringIO()
                        with patch('sys.argv', ['publisher', '--report', str(root/'report'),
                                               '--acceptance', str(root/'acceptance'), '--grant', str(root/'grant'),
                                               '--candidate', candidate['repository']]), \
                                patch.object(publisher.site, 'check_report', return_value=report), \
                                patch('publisher.credential') as credential, \
                                patch('publisher.transfer') as transfer, contextlib.redirect_stderr(stderr):
                            self.assertEqual(publisher.cli(), 1)
                            credential.assert_not_called()
                            transfer.assert_not_called()
                        self.assertEqual(stderr.getvalue(), 'website publisher: refused or unconfirmed; review local approval and remote state\n')


class RemoteBoundaryTests(unittest.TestCase):
    def test_only_successful_exact_ref_query_establishes_absence(self):
        from pathlib import Path
        import subprocess
        from unittest.mock import patch
        cases = [(0, b'', None),
                 (0, b'1111111111111111111111111111111111111111\trefs/heads/gh-pages\n', '1'*40)]
        for status, output, expected in cases:
            with patch('publisher.subprocess.run', return_value=subprocess.CompletedProcess([],status,output,b'')):
                self.assertEqual(publisher.remote_tip(Path('/tmp'), 'fixture-secret'), expected)
        for status, output in [(1,b''), (0,b' '), (0,b'1'*40+b'\trefs/heads/main\n'),
                               (0,b'1'*40+b'\trefs/heads/gh-pages\nextra')]:
            with self.subTest(status=status, output=output), \
                    patch('publisher.subprocess.run', return_value=subprocess.CompletedProcess([],status,output,b'fixture-secret')):
                with self.assertRaises(ValueError) as failure:
                    publisher.remote_tip(Path('/tmp'), 'fixture-secret')
                self.assertNotIn('fixture-secret', str(failure.exception))


if __name__ == '__main__':
    unittest.main()
