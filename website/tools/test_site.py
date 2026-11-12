# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Behavioral fault injection using disposable source/output fixtures."""
import copy
import importlib.util
import json
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('site_tools', Path(__file__).with_name('site.py'))
site = importlib.util.module_from_spec(spec)
spec.loader.exec_module(site)


class InventoryTests(unittest.TestCase):
    def test_symlink_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'a').write_text('safe')
            (root / 'b').symlink_to(root / 'a')
            with self.assertRaisesRegex(ValueError, 'symlink'):
                site.inventory(root)

    def test_content_change_changes_digest(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'a').write_text('before')
            first = site.encoded_hash(site.inventory(root))
            (root / 'a').write_text('after')
            self.assertNotEqual(first, site.encoded_hash(site.inventory(root)))


class OutputTests(unittest.TestCase):
    @staticmethod
    def policy(content=None):
        return f'<meta http-equiv="Content-Security-Policy" content="{content or site.CSP}">'

    def fixture(self, root):
        for route in site.ROUTES:
            page = root / (route.lstrip('/') + 'index.html')
            page.parent.mkdir(parents=True, exist_ok=True)
            canonical = site.BASE + route.lstrip('/')
            page.write_text(f'{self.policy()}<link rel="canonical" href="{canonical}"><main><h1>Fixture</h1><aside data-status="development"></aside><a href="/GoPMgr/docs/">Docs</a></main>')
        (root / '404.html').write_text(f'{self.policy()}<link rel="canonical" href="{site.BASE}404.html"><main><h1>Missing</h1></main>')
        (root / '.nojekyll').touch()
        (root / 'flexsearch.js').write_text('// fixture')
        (root / 'search.json').write_text('{}')
        search = {site.PREFIX + route.lstrip('/'): {'title': 'Fixture · ' + ('Historical' if route == '/downloads/' else 'Planned' if route == '/roadmap/' else 'Development')} for route in site.ROUTES[1:]}
        (root / 'en.search-data.json').write_text(json.dumps(search))
        locations = [site.BASE + route.lstrip('/') for route in site.ROUTES]
        urls = ''.join(f'<url><loc>{location}</loc></url>' for location in locations)
        (root / 'sitemap.xml').write_text('<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">' + urls + '</urlset>')

    def test_valid_fixture(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self.fixture(root)
            self.assertIn('index.html', site.output_check(root))

    def test_faults(self):
        mutations = {
            'external resource': '<script src="https://example.org/a.js"></script>',
            'prefix escape': '<a href="/docs/">Bad</a>',
            'missing target': '<a href="/GoPMgr/missing/">Bad</a>',
            'missing anchor': '<a href="/GoPMgr/docs/#absent">Bad</a>',
            'missing alt': '<img src="/GoPMgr/search.json">',
            'frame': '<iframe src="https://example.org"></iframe>',
            'event': '<button onclick="evil()">Bad</button>',
            'unsafe URL': '<a href="javascript:evil()">Bad</a>',
        }
        for label, fragment in mutations.items():
            with self.subTest(label=label), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                self.fixture(root)
                page = root / 'index.html'
                page.write_text(page.read_text() + fragment)
                with self.assertRaises(ValueError):
                    site.output_check(root)

    def test_page_policy_faults_name_their_cause(self):
        """Each page carries exactly the reviewed policy and nothing that defeats it."""
        added = {
            'inline script': '<script>evil()</script>',
            'base element': '<base href="https://example.org/">',
            'generator version disclosed': '<meta name="generator" content="Hugo 0.169.0">',
            'placeholder image alt': '<img src="/GoPMgr/search.json" alt="Dark Logo">',
            'content security policy': self.policy("default-src *"),
        }
        for message, fragment in added.items():
            with self.subTest(message=message), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                self.fixture(root)
                page = root / 'docs/index.html'
                page.write_text(page.read_text() + fragment)
                with self.assertRaisesRegex(ValueError, message):
                    site.output_check(root)
        replaced = {
            'absent': '',
            'script from any origin': self.policy(site.CSP.replace("script-src 'self'", 'script-src *')),
            'inline script allowed': self.policy(site.CSP.replace("script-src 'self'", "script-src 'self' 'unsafe-inline'")),
        }
        for label, policy in replaced.items():
            for name in ('index.html', '404.html'):
                with self.subTest(label=label, name=name), tempfile.TemporaryDirectory() as tmp:
                    root = Path(tmp)
                    self.fixture(root)
                    page = root / name
                    page.write_text(page.read_text().replace(self.policy(), policy))
                    with self.assertRaisesRegex(ValueError, name + ': missing or altered content security policy'):
                        site.output_check(root)

    def test_stale_output_and_css_network(self):
        for name, value in [('stale.html', '<h1>Stale</h1>'), ('x.css', 'a{background:url(https://example.org/a)}')]:
            with self.subTest(name=name), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                self.fixture(root)
                (root / name).write_text(value)
                with self.assertRaises(ValueError):
                    site.output_check(root)


class SourceLedgerTests(unittest.TestCase):
    def test_dependency_and_inventory_faults(self):
        import shutil
        original_site, original_root = site.SITE, site.ROOT
        ledger = json.loads((original_site / 'quality/source-dependencies.json').read_text())
        dependencies = {dep['path'] for page in ledger['pages'] for dep in page['dependencies']}
        mutations = ('canonical drift', 'extra content', 'missing content', 'duplicate claim',
                     'template drift', 'renamed content', 'duplicate route', 'stale snapshot label')
        for mutation in mutations:
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                copied_site = root / 'website'
                shutil.copytree(original_site, copied_site, ignore=shutil.ignore_patterns('resources', '__pycache__', '.hugo_build.lock'))
                for name in dependencies:
                    path = root / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copyfile(original_root / name, path)
                # Review the fixture as-is before injecting a single defect.
                with patch.object(site, 'SITE', copied_site), patch.object(site, 'ROOT', root):
                    reviewed = copied_site / 'quality/reviewed-inputs.json'
                    reviewed.write_text(json.dumps({'files': site.input_inventory()}))
                    expected = {
                        'canonical drift': 'canonical source drift',
                        'extra content': 'unreviewed or missing',
                        'missing content': 'No such file',
                        'renamed content': 'No such file',
                        'duplicate claim': 'duplicate claim',
                        'duplicate route': 'duplicate route',
                        'template drift': 'implementation inputs changed',
                        'stale snapshot label': 'stale snapshot label',
                    }[mutation]
                    if mutation == 'stale snapshot label':
                        # Re-register afterwards so only the label check can reject it.
                        page = copied_site / 'content/features/_index.md'
                        page.write_text(page.read_text().replace(ledger['canonical_commit'][:12], '0' * 12, 1))
                        reviewed.write_text(json.dumps({'files': site.input_inventory()}))
                    elif mutation == 'canonical drift':
                        (root / sorted(dependencies)[0]).write_text('changed source')
                    elif mutation == 'extra content':
                        (copied_site / 'content/unreviewed.md').write_text('unreviewed')
                    elif mutation == 'missing content':
                        (copied_site / 'content/features/_index.md').unlink()
                    elif mutation == 'renamed content':
                        (copied_site / 'content/features/_index.md').rename(copied_site / 'content/features/new.md')
                    elif mutation in ('duplicate claim', 'duplicate route'):
                        changed = copy.deepcopy(ledger)
                        key = 'claim_ids' if mutation == 'duplicate claim' else 'route'
                        changed['pages'][1][key] = changed['pages'][0][key]
                        (copied_site / 'quality/source-dependencies.json').write_text(json.dumps(changed))
                        if mutation == 'duplicate claim':
                            page = copied_site / 'content/features/_index.md'
                            page.write_text(page.read_text().replace(
                                'claim_ids = ["page.features"]', 'claim_ids = ["page.home"]', 1))
                    else:
                        with (copied_site / 'layouts/home.html').open('a') as stream:
                            stream.write('unreviewed behavior')
                    # Git resolves against the real frozen canonical repository;
                    # only fixture bytes are mutated, never the checkout sources.
                    actual_output = site.subprocess.check_output
                    def git_output(args, **kwargs):
                        return actual_output(args, cwd=original_root)
                    with patch.object(site.subprocess, 'check_output', side_effect=git_output):
                        with self.assertRaisesRegex((ValueError, FileNotFoundError), expected):
                            site.canonical_check()

class IdentityTests(unittest.TestCase):
    def test_mode_changes_identity(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'script'
            path.write_text('same bytes')
            path.chmod(0o644)
            before = site.file_record(path)
            path.chmod(0o755)
            self.assertNotEqual(before, site.file_record(path))

    def test_hardlink_rejected(self):
        import os
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / 'a').write_text('same inode')
            os.link(root / 'a', root / 'b')
            with self.assertRaisesRegex(ValueError, 'hardlink'):
                site.inventory(root)

    def test_executable_public_output_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            OutputTests().fixture(root)
            (root / 'index.html').chmod(0o755)
            with self.assertRaisesRegex(ValueError, 'mode 100644'):
                site.output_check(root)

    def test_ambient_hugo_override_rejected(self):
        with patch.object(site, 'canonical_check'), patch.object(site, 'snapshot'), patch.dict(site.os.environ, {'HUGO_TITLE': 'Unreviewed', 'HUGO': str(site.ROOT / '.tmp/website-tools/bin/hugo')}):
            with self.assertRaisesRegex(ValueError, 'ambient HUGO_'):
                site.build()

    def test_full_snapshot_tracks_new_license_and_website_modes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copied = root / 'website'
            (copied / 'quality').mkdir(parents=True)
            (copied / 'quality/reviewed-inputs.json').write_text('{}')
            script = copied / 'script.sh'
            script.write_text('fixture')
            script.chmod(0o644)
            (root / 'LICENSES').mkdir()
            license_file = root / 'LICENSES/Apache-2.0.txt'
            license_file.write_text('license A')
            (root / 'Makefile').write_text('fixture')
            def git_result(args, **kwargs):
                if 'ls-files' in args:
                    return 'Makefile'
                if 'rev-parse' in args:
                    return '0' * 40
                return '?? website/'
            with patch.object(site, 'ROOT', root), patch.object(site, 'SITE', copied), patch.object(site, 'run', side_effect=git_result):
                before = site.snapshot()['digest']
                license_file.write_text('license B')
                after_license = site.snapshot()['digest']
                self.assertNotEqual(before, after_license)
                script.chmod(0o755)
                self.assertNotEqual(after_license, site.snapshot()['digest'])

class AcceptanceTests(unittest.TestCase):
    def test_required_gates_and_identity(self):
        report = {'source': {'digest': 'source fixture'}, 'output_digest': 'output fixture'}
        acceptance = {'schema_version': 1, 'reviewer': 'TEST FIXTURE ONLY',
                      'reviewed_at': '2026-11-10', 'source_digest': 'source fixture',
                      'output_digest': 'output fixture', 'private_channel': 'TEST FIXTURE',
                      'owner_channel_approval': 'TEST FIXTURE: no real approval',
                      'gates': {**{f'A{i:02}': 'PASS' for i in range(1, 16)},
                                **{f'R{i}': 'PASS' for i in range(1, 12)}}}
        site.acceptance_check(report, acceptance)
        for gate in acceptance['gates']:
            for state in ('BLOCKED', 'FAIL', 'NOT RUN'):
                changed = copy.deepcopy(acceptance)
                changed['gates'][gate] = state
                with self.subTest(gate=gate, state=state), self.assertRaisesRegex(ValueError, 'not all PASS'):
                    site.acceptance_check(report, changed)
        for field in ('source_digest', 'output_digest'):
            changed = copy.deepcopy(acceptance)
            changed[field] = 'changed identity'
            with self.assertRaisesRegex(ValueError, 'different source/output'):
                site.acceptance_check(report, changed)
        changed = copy.deepcopy(acceptance)
        changed['private_channel'] = ''
        with self.assertRaisesRegex(ValueError, 'channel missing'):
            site.acceptance_check(report, changed)

class MetadataTests(unittest.TestCase):
    def test_canonical_sitemap_and_search_defects(self):
        for name in ('canonical', 'sitemap', 'search status'):
            with self.subTest(name=name), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                OutputTests().fixture(root)
                if name == 'canonical':
                    page = root / 'index.html'
                    page.write_text(page.read_text().replace(site.BASE, 'https://example.com/'))
                    expected = 'incorrect canonical'
                elif name == 'sitemap':
                    path = root / 'sitemap.xml'
                    path.write_text(path.read_text().replace(site.BASE, 'https://example.com/'))
                    expected = 'sitemap URL inventory'
                else:
                    path = root / 'en.search-data.json'
                    path.write_text(path.read_text().replace('Development', 'Released'))
                    expected = 'search result missing applicability'
                with self.assertRaisesRegex(ValueError, expected):
                    site.output_check(root)


class PublicClaimTests(unittest.TestCase):
    def test_downloads_page_matches_release_record(self):
        page = (site.SITE / 'content/downloads/_index.md').read_text()
        body = page.split('+++', 2)[-1]
        self.assertNotIn('ad-hoc', body)
        self.assertIn('unsigned', body)
        self.assertIn('not notarized', body)
        ledger = json.loads((site.SITE / 'quality/source-dependencies.json').read_text())
        downloads = next(item for item in ledger['pages'] if item['route'] == '/downloads/')
        paths = [dep['path'] for dep in downloads['dependencies']]
        self.assertIn('docs/releases/v1.1.0-alpha.2.md', paths)

    def test_public_credits_match_vendored_notices(self):
        hextra = (site.SITE / 'static/licenses/Hextra.txt').read_text()
        bundle = (site.SITE / 'assets/js/vendor/flexsearch.bundle.min.js').read_text()
        self.assertIn('Copyright (c) 2023 Xin', hextra)
        self.assertIn('Author and Copyright: Thomas Wilkerling', bundle)
        reuse = (site.ROOT / 'REUSE.toml').read_text()
        legal = (site.SITE / 'content/legal/_index.md').read_text()
        self.assertIn('2023 Xin', reuse)
        self.assertIn('Thomas Wilkerling', reuse)
        self.assertNotIn('Hextra contributors', reuse)
        self.assertNotIn('Nextapps GmbH', reuse)
        self.assertIn('Xin', legal)
        self.assertIn('Thomas Wilkerling', legal)
        self.assertIn('Nextapps', legal)

    def test_repository_blob_links_are_reviewed_dependencies(self):
        ledger = json.loads((site.SITE / 'quality/source-dependencies.json').read_text())
        pattern = re.compile(
            r'https://github.com/draconianneocoder-cpu/GoPMgr/blob/([0-9a-f]{40})/([^)\s\"\']+)')
        for page in ledger['pages']:
            text = (site.ROOT / page['content_path']).read_text()
            deps = {dep['path']: dep['reviewed_commit'] for dep in page['dependencies']}
            for sha, raw in pattern.findall(text):
                path = raw.split('#', 1)[0]
                self.assertEqual(deps.get(path), sha, page['content_path'] + ' ' + path)

    def test_changed_source_link_commit_is_rejected(self):
        import shutil
        original_site, original_root = site.SITE, site.ROOT
        ledger = json.loads((original_site / 'quality/source-dependencies.json').read_text())
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copied = root / 'website'
            shutil.copytree(original_site, copied, ignore=shutil.ignore_patterns(
                'resources', '__pycache__', '.hugo_build.lock'))
            for dep in {item['path'] for page in ledger['pages'] for item in page['dependencies']}:
                dest = root / dep
                dest.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(original_root / dep, dest)
            page = copied / 'content/_index.md'
            text = page.read_text()
            # Read the pin from the ledger so a re-pin cannot turn this into a no-op.
            self.assertIn(ledger['canonical_commit'], text)
            page.write_text(text.replace(ledger['canonical_commit'], '0' * 40, 1))
            with patch.object(site, 'SITE', copied), patch.object(site, 'ROOT', root):
                (copied / 'quality/reviewed-inputs.json').write_text(
                    json.dumps({'files': site.input_inventory()}))
                actual = site.subprocess.check_output

                def git_output(args, **kwargs):
                    return actual(args, cwd=original_root)

                with patch.object(site.subprocess, 'check_output', side_effect=git_output):
                    with self.assertRaisesRegex(ValueError, 'source link is not a reviewed dependency'):
                        site.canonical_check()

    def test_getting_started_keeps_old_recovery_codes_until_confirmation(self):
        page = (site.SITE / 'content/docs/getting-started/_index.md').read_text()
        body = page.split('+++', 2)[-1]
        self.assertNotIn('invalidates older codes', body)
        self.assertIn('Use the new codes', body)

    def test_security_page_and_policy_name_the_github_report_form(self):
        url = 'https://github.com/draconianneocoder-cpu/GoPMgr/security/advisories/new'
        page = (site.SITE / 'content/security/_index.md').read_text()
        policy = (site.ROOT / 'docs/SECURITY.md').read_text()
        self.assertIn(url, page)
        self.assertIn(url, policy)

    def test_security_reporting_is_not_described_as_pending(self):
        page = (site.SITE / 'content/security/_index.md').read_text()
        policy = (site.ROOT / 'docs/SECURITY.md').read_text()
        readme = (site.SITE / 'README.md').read_text()
        checker = (site.SITE / 'tools/site.py').read_text()
        self.assertNotIn('Private vulnerability reporting: pending', page)
        self.assertNotIn('does not currently publish a public vulnerability intake', policy)
        self.assertNotIn('security-reporting channel is pending', readme)
        self.assertNotIn('Approved private disclosure channel is pending.', checker)

    def test_website_targets_are_in_make_help(self):
        lines = (site.ROOT / 'Makefile').read_text().splitlines()
        for target in ('website-build', 'website-check', 'website-prepare-publication'):
            matched = [line for line in lines if line.startswith(target + ':') and '##' in line]
            self.assertTrue(matched, target)


if __name__ == '__main__':
    unittest.main()
