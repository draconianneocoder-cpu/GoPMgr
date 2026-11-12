#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
# SPDX-License-Identifier: GPL-3.0-or-later
"""Local website build and verification. No remote writes or settings operations."""
import argparse
import hashlib
from html.parser import HTMLParser
import json
import os
from pathlib import Path
import platform
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import tomllib
import xml.etree.ElementTree as ET
from urllib.parse import unquote, urlsplit, urljoin

SITE = Path(__file__).resolve().parents[1]
ROOT = SITE.parent
ROUTES = ('/', '/features/', '/docs/', '/docs/getting-started/',
          '/docs/getting-started/first-project/', '/downloads/', '/roadmap/',
          '/community/', '/security/', '/legal/')
PREFIX = '/GoPMgr/'
BASE = 'https://draconianneocoder-cpu.github.io' + PREFIX
# Must match layouts/_partials/custom/head-end.html. A page that loads script,
# styles or data from another origin, or runs inline script, breaks under it.
CSP = ("default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; "
       "img-src 'self' data:; font-src 'self'; connect-src 'self'; base-uri 'none'; form-action 'none'")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def encoded_hash(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def run(args, cwd=ROOT, env=None):
    return subprocess.check_output(args, cwd=cwd, env=env, text=True).strip()


def file_record(path):
    require(path.is_file() and not path.is_symlink(), f'not a regular input: {path}')
    info = path.stat()
    require(info.st_nlink == 1, f'hardlink input: {path}')
    return {'sha256': digest(path), 'size': info.st_size,
            'mode': '100755' if info.st_mode & 0o111 else '100644'}


def inventory(directory):
    """Reject symlinks, special files and case aliases before hashing actual bytes."""
    require(directory.is_dir() and not directory.is_symlink(), f'inventory root must be a real directory: {directory}')
    result, folded = {}, set()
    for path in sorted(directory.rglob('*')):
        require(not path.is_symlink(), f'symlink forbidden: {path}')
        relative = path.relative_to(directory).as_posix()
        require(relative.casefold() not in folded, f'case alias: {relative}')
        folded.add(relative.casefold())
        if path.is_dir():
            continue
        info = path.stat()
        require(stat.S_ISREG(info.st_mode), f'not a regular file: {path}')
        require(info.st_nlink == 1, f'hardlink forbidden: {path}')
        result[relative] = {'sha256': digest(path), 'size': info.st_size,
                            'mode': '100755' if info.st_mode & 0o111 else '100644'}
    return result


def input_inventory():
    # Independent enumeration: adding an unregistered page or template changes this digest.
    files = inventory(SITE)
    return {name: record for name, record in files.items()
            if not name.startswith(('__pycache__/', 'tools/__pycache__/', 'resources/'))
            and name not in ('quality/reviewed-inputs.json', '.hugo_build.lock')}


def canonical_check():
    ledger = json.loads((SITE / 'quality/source-dependencies.json').read_text())
    require(ledger['schema_version'] == 1, 'unknown source ledger schema')
    routes, paths, claims = set(), set(), set()
    for page in ledger['pages']:
        route, name = page['route'], page['content_path']
        require(route not in routes and name not in paths, 'duplicate route or content path')
        routes.add(route)
        paths.add(name)
        path = ROOT / name
        require(path.is_relative_to(SITE / 'content') and '..' not in Path(name).parts,
                'content path escapes website/content')
        require(not path.is_symlink(), 'content symlink')
        text = path.read_text()
        front = tomllib.loads(text.split('+++', 2)[1])['params']
        for key in ('claim_ids', 'status', 'last_reviewed'):
            require(front[key] == page[key], f'ledger/front matter mismatch: {name} {key}')
        require(page['status'] in ('development', 'historical', 'planned'), 'unclassified capability')
        # The banner names the snapshot readers are told the page describes.
        require(page['status'] == 'historical' or ledger['canonical_commit'][:12] in front.get('applies_to', ''),
                f'stale snapshot label: {name}')
        require(front.get('applies_to') and front.get('sources') and page['reviewer'], 'missing review metadata')
        for claim in page['claim_ids']:
            require(claim not in claims, f'duplicate claim: {claim}')
            claims.add(claim)
        require(page['dependencies'], 'missing canonical dependency')
        for dep in page['dependencies']:
            source = ROOT / dep['path']
            require('..' not in Path(dep['path']).parts and not source.is_symlink(), 'unsafe dependency')
            require(source.is_file() and digest(source) == dep['sha256'], f'canonical source drift: {dep["path"]}')
            historical = subprocess.check_output(['git', 'show', f'{dep["reviewed_commit"]}:{dep["path"]}'], cwd=ROOT)
            require(hashlib.sha256(historical).hexdigest() == dep['sha256'], 'reviewed commit does not match source')
        for sha, raw in re.findall(
                r'https://github.com/draconianneocoder-cpu/GoPMgr/blob/([0-9a-f]{40})/([^)\s\"\']+)', text):
            cited = raw.split('#', 1)[0]
            require(any(dep['path'] == cited and dep['reviewed_commit'] == sha for dep in page['dependencies']),
                    f'source link is not a reviewed dependency: {name} {cited}')
    actual = {p.relative_to(ROOT).as_posix() for p in (SITE / 'content').rglob('*.md')}
    require(paths == actual, 'unreviewed or missing content file')
    require(routes == set(ROUTES), 'required route inventory mismatch')
    pins = json.loads((SITE / 'quality/dependencies.json').read_text())
    for name, expected in pins['files'].items():
        require(digest(SITE / name) == expected, f'vendored asset drift: {name}')
    reviewed = json.loads((SITE / 'quality/reviewed-inputs.json').read_text())
    require(reviewed['files'] == input_inventory(), 'implementation inputs changed since recorded review')


class Page(HTMLParser):
    def __init__(self, text):
        super().__init__(convert_charrefs=True)
        self.ids, self.refs, self.resources, self.canonical, self.csp = set(), [], [], [], []
        self.h1 = self.main = self.status = 0
        self.errors = []
        self.feed(text)

    def handle_starttag(self, tag, attributes):
        a = dict(attributes)
        if 'id' in a:
            if a['id'] in self.ids:
                self.errors.append('duplicate id ' + a['id'])
            self.ids.add(a['id'])
        self.h1 += tag == 'h1'
        self.main += tag == 'main'
        self.status += 'data-status' in a
        if tag == 'img' and 'alt' not in a:
            self.errors.append('image missing alt')
        if tag == 'img' and (a.get('alt') or '').strip().lower() in ('logo', 'dark logo'):
            self.errors.append('placeholder image alt')
        if tag in ('iframe', 'object', 'embed') or tag == 'form':
            self.errors.append('unexpected embedded content or form')
        if tag == 'link' and a.get('rel') == 'canonical':
            self.canonical.append(a.get('href'))
        if tag == 'a' and a.get('href'):
            self.refs.append(a['href'])
        if tag == 'script' and a.get('src'):
            self.resources.append(a['src'])
        elif tag == 'script':
            self.errors.append('inline script')
        if tag == 'base':
            self.errors.append('base element')
        if tag == 'meta' and (a.get('http-equiv') or '').lower() == 'content-security-policy':
            self.csp.append(a.get('content'))
        if tag == 'meta' and (a.get('name') or '').lower() == 'generator':
            self.errors.append('generator version disclosed')
        if tag in ('img', 'source', 'audio', 'video') and a.get('src'):
            self.resources.append(a['src'])
        if a.get('srcset'):
            self.resources.extend(x.strip().split()[0] for x in a['srcset'].split(','))
        if tag == 'link' and a.get('rel') not in ('canonical', 'alternate') and a.get('href'):
            self.resources.append(a['href'])
        if any(k.startswith('on') for k in a):
            self.errors.append('inline event handler')


def output_check(public):
    files = inventory(public)
    require('.nojekyll' in files, 'missing .nojekyll')
    require(all(item['mode'] == '100644' for item in files.values()), 'public output must contain only mode 100644 files')
    pages = {name: Page((public / name).read_text()) for name in files if name.endswith('.html')}
    expected = {route.lstrip('/') + 'index.html' for route in ROUTES} | {'404.html'}
    require(set(pages) == expected, 'unexpected or missing generated HTML routes')
    for name, page in pages.items():
        require(not page.errors, f'{name}: {page.errors}')
        canonical = BASE + (name[:-10] if name.endswith('index.html') else name)
        require(page.canonical == [canonical], f'{name}: incorrect canonical URL')
        require(page.h1 == 1 and page.main == 1, f'{name}: expected one h1 and main')
        require(page.csp == [CSP], f'{name}: missing or altered content security policy')
        require(name == '404.html' or page.status == 1, f'{name}: missing rendered status')
        for ref in page.resources:
            require(not urlsplit(ref).scheme and not ref.startswith('//'), f'external page-load resource: {ref}')
        for ref in page.refs + page.resources:
            parsed = urlsplit(ref)
            require(parsed.scheme in ('', 'https', 'mailto'), f'unsafe URL: {ref}')
            if parsed.scheme or parsed.netloc:
                continue
            target = unquote(parsed.path)
            if target and not target.startswith('/'):
                page_url = PREFIX + (name[:-10] if name.endswith('index.html') else name)
                target = urlsplit(urljoin(page_url, target)).path
            if not target:
                relative = name
            else:
                require(target.startswith(PREFIX), f'link outside project prefix: {name}: {ref}')
                relative = target[len(PREFIX):]
                require('..' not in Path(relative).parts, 'path traversal')
                if not relative or relative.endswith('/'):
                    relative += 'index.html'
            require(relative in files, f'broken local link: {name}: {ref}')
            if parsed.fragment and relative in pages:
                require(unquote(parsed.fragment) in pages[relative].ids, f'broken anchor: {name}: {ref}')
    for name in files:
        require(not any(part in name.lower() for part in ('.git/', '.agent_memory', 'session-notes', '.env', '.map', 'service-worker')), f'private/unexpected output: {name}')
        if name.endswith('.css'):
            css = (public / name).read_text()
            require(not re.search(r'@import|url\(\s*[\'\"]?(?:https?:|//)', css, re.I), f'external CSS resource: {name}')
    require(any('flexsearch' in n and n.endswith('.js') for n in files), 'missing local search runtime')
    require(any('search' in n and n.endswith('.json') for n in files), 'missing local search index')
    indexes = [name for name in files if 'search-data' in name and name.endswith('.json')]
    require(len(indexes) == 1, 'unexpected search index inventory')
    search = json.loads((public / indexes[0]).read_text())
    require(set(search) == {PREFIX + route.lstrip('/') for route in ROUTES[1:]}, 'search route inventory mismatch')
    for route, entry in search.items():
        status = 'Historical' if route == PREFIX + 'downloads/' else 'Planned' if route == PREFIX + 'roadmap/' else 'Development'
        require(entry.get('title', '').endswith(' · ' + status), f'search result missing applicability: {route}')
    locations = {node.text for node in ET.parse(public / 'sitemap.xml').iter() if node.tag.endswith('}loc')}
    require(locations == {BASE + route.lstrip('/') for route in ROUTES}, 'sitemap URL inventory mismatch')
    return files


def snapshot():
    paths = run(['git', 'ls-files', '-z']).split('\0')
    records = {}
    for name in paths:
        if not name:
            continue
        path = ROOT / name
        require(not path.is_symlink(), f'tracked symlink: {name}')
        records[name] = file_record(path) if path.is_file() else {'deleted': True}
    for name, record in input_inventory().items():
        records['website/' + name] = record
    reviewed = SITE / 'quality/reviewed-inputs.json'
    records['website/quality/reviewed-inputs.json'] = file_record(reviewed)
    # Named here so the snapshot fails if this distribution license goes missing.
    records['LICENSES/Apache-2.0.txt'] = file_record(ROOT / 'LICENSES/Apache-2.0.txt')
    return {'head': run(['git', 'rev-parse', 'HEAD']),
            'status': run(['git', 'status', '--porcelain=v1', '--untracked-files=all']),
            'files': records, 'digest': encoded_hash(records)}


def build():
    require(not any(key.startswith('HUGO_') for key in os.environ), 'ambient HUGO_* configuration overrides are forbidden')
    canonical_check()
    binary = shutil.which(os.environ.get('HUGO', 'hugo'))
    require(binary, 'Hugo unavailable; set HUGO to the verified task-local executable')
    pins = json.loads((SITE / 'quality/dependencies.json').read_text())
    key = platform.system().lower() + '/' + {'aarch64': 'arm64', 'x86_64': 'amd64'}.get(platform.machine(), platform.machine())
    pin = pins['hugo']['platforms'].get(key)
    require(pin and digest(Path(binary)) == pin['executable_sha256'], 'Hugo executable provenance not approved for this platform')
    version = run([binary, 'version'])
    require(version.startswith('hugo v0.169.0-') and '+extended ' in version, 'incorrect Hugo version/edition')
    env = dict(os.environ, GOWORK='off')
    before = {name: digest(SITE / name) for name in ('go.mod', 'go.sum')}
    source = snapshot()
    run(['go', 'mod', 'verify'], cwd=SITE, env=env)
    graph = run([binary, 'mod', 'graph'], cwd=SITE, env=env)
    require(graph.splitlines() == ['github.com/draconianneocoder-cpu/GoPMgr/website github.com/imfing/hextra@v0.13.0'], 'unexpected module graph: ' + graph)
    scratch = ROOT / '.tmp'
    scratch.mkdir(exist_ok=True)
    out = Path(tempfile.mkdtemp(prefix='website-build-', dir=scratch))
    public = out / 'public'
    run([binary, '--environment', 'production', '--cacheDir', str(out / 'cache'), '--destination', str(public), '--gc', '--minify', '--panicOnWarning'], cwd=SITE, env=env)
    (public / '.nojekyll').touch()
    require(before == {name: digest(SITE / name) for name in before}, 'module lockfiles changed during build')
    require(source['digest'] == snapshot()['digest'], 'source changed during build')
    files = output_check(public)
    report = {'schema_version': 1, 'public': str(public), 'source': source,
              'output': files, 'output_digest': encoded_hash(files), 'hugo': version,
              'hugo_env': run([binary, 'env']), 'go': run(['go', 'version']),
              'stage_d': 'PASS', 'stage_p': 'BLOCKED',
              'blockers': ['Full publication acceptance and owner authorization are not recorded.']}
    (out / 'build-report.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'public': str(public), 'report': str(out / 'build-report.json'),
                      'output_digest': report['output_digest'], 'stage_p': report['stage_p']}, indent=2))


def check_report(path):
    report = json.loads(path.read_text())
    canonical_check()
    require(report['source']['digest'] == snapshot()['digest'], 'build source snapshot is stale')
    files = output_check(Path(report['public']))
    require(files == report['output'] and encoded_hash(files) == report['output_digest'], 'built artifact changed')
    return report


def acceptance_check(report, acceptance):
    require(acceptance.get('schema_version') == 1 and acceptance.get('reviewer') and acceptance.get('reviewed_at'), 'invalid acceptance record')
    require(acceptance.get('source_digest') == report['source']['digest'] and acceptance.get('output_digest') == report['output_digest'], 'acceptance refers to different source/output bytes')
    required = {f'A{i:02}' for i in range(1, 16)} | {f'R{i}' for i in range(1, 12)}
    gates = acceptance.get('gates', {})
    require(set(gates) == required and all(value == 'PASS' for value in gates.values()), 'required publication gates are not all PASS')
    require(acceptance.get('owner_channel_approval') and acceptance.get('private_channel'), 'approved private disclosure channel missing')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('command', choices=('build', 'check', 'prepare-publication'))
    parser.add_argument('--report', type=Path)
    parser.add_argument('--acceptance', type=Path)
    parser.add_argument('--parent-repository', type=Path)
    parser.add_argument('--expected-tip')
    args = parser.parse_args()
    if args.command == 'build':
        build()
    else:
        require(args.report, '--report is required; checks never select an ambiguous latest artifact')
        report = check_report(args.report)
        if args.command == 'prepare-publication':
            require(args.acceptance, 'publication preparation BLOCKED: approved private channel and artifact-bound acceptance record required')
            acceptance = json.loads(args.acceptance.read_text())
            acceptance_check(report, acceptance)
            from publication import stage
            receipt = stage(Path(report['public']), report['output'], inventory, args.parent_repository, args.expected_tip)
            print(json.dumps(receipt, indent=2))
            return
        print('PASS: canonical dependencies, reviewed inputs, source snapshot and exact generated artifact')


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, KeyError, ET.ParseError, subprocess.CalledProcessError) as error:
        print(f'website: {error}', file=sys.stderr)
        sys.exit(1)
