<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import type { SectionId } from '../help-sections';

  let { active, nav }: { active: SectionId; nav: (id: SectionId) => void } = $props();
</script>

        <!-- ── Troubleshooting & FAQ ──────────────────────────────── -->
        {#if active === 'troubleshooting'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Troubleshooting &amp; FAQ</h2>
          <p class="text-sm text-slate-400 mb-5">
            The most common problems and their fixes. If your issue isn't here, the search box
            above the sidebar covers every section of this guide.
          </p>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">I forgot my passphrase</h3>
            <p class="text-sm text-slate-300">
              On the login screen choose <span class="font-medium text-slate-100">"Forgot password? Use a recovery code"</span>,
              then enter your username, one unused recovery code, and a new passphrase. Each code
              works once; reissue a fresh set afterwards from App Settings. If the passphrase
              <span class="font-medium text-slate-100">and</span> all recovery codes are lost,
              encrypted project databases are unrecoverable by design — there is no back door.
            </p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">There is no administrator</h3>
            <p class="text-sm text-slate-300">
              If the first account skipped the admin claim, open
              <button onclick={() => nav('app-settings')} class="text-cyan-400 underline hover:text-cyan-300">App Settings</button>
              and use <span class="font-medium text-slate-100">Become administrator</span> — available
              while no other admin exists on the machine.
            </p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">GoPMgr won't start</h3>
            <p class="text-sm text-slate-300">
              Startup failures are written to a dated log and a native error dialog names the log
              path. When the app does run, App Settings &rarr;
              <span class="font-medium text-slate-100">Open Logs Folder</span> takes you to the same
              directory. On Linux, a blank or missing window usually means the WebKit runtime is
              absent — see <button onclick={() => nav('install')} class="text-cyan-400 underline hover:text-cyan-300">Installing &amp; Running</button>
              for the required packages.
            </p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">A project may be corrupt</h3>
            <p class="text-sm text-slate-300">
              Run the built-in maintenance from a terminal:
              <span class="font-mono text-xs">--check</span> reports integrity,
              <span class="font-mono text-xs">--repair</span> runs the self-healing workflow and
              prints what it did, and <span class="font-mono text-xs">--vacuum</span> compacts the
              file. See <button onclick={() => nav('cli')} class="text-cyan-400 underline hover:text-cyan-300">Command-Line Maintenance</button>
              for exact invocations (encrypted projects also need
              <span class="font-mono text-xs">--username</span> / <span class="font-mono text-xs">--password-env</span>).
            </p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">My exported PDF isn't signed</h3>
            <p class="text-sm text-slate-300">
              The Signature Options dialog at export decides: PAdES needs a
              <span class="font-mono text-xs">.p12</span>/<span class="font-mono text-xs">.pfx</span>
              certificate and its password; GnuPG writes a detached
              <span class="font-mono text-xs">.asc</span> sidecar next to the PDF (verify with
              <span class="font-mono text-xs">gpg --verify file.pdf.asc file.pdf</span>); "No digital
              signature" produces a plain PDF. Set the project default in
              <button onclick={() => nav('project-settings')} class="text-cyan-400 underline hover:text-cyan-300">Project Settings</button>;
              details in <button onclick={() => nav('export-signing')} class="text-cyan-400 underline hover:text-cyan-300">Export &amp; Digital Signing</button>.
            </p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Exports have missing or wrong-looking text</h3>
            <p class="text-sm text-slate-300">
              PDF output embeds TrueType fonts. If glyphs are missing, fetch the bundled catalog
              (<span class="font-mono text-xs">make fonts</span> in a source checkout) or import a
              <span class="font-mono text-xs">.ttf</span> of your choice from Project Settings and
              select it for documents.
            </p>
          </section>

          <section class="mb-5">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">The Kanban / Sprint views are missing</h3>
            <p class="text-sm text-slate-300">
              The agile pack appears when the project's methodology enables it (Scrum, Kanban,
              Scrumban). Change the methodology in
              <button onclick={() => nav('project-settings')} class="text-cyan-400 underline hover:text-cyan-300">Project Settings</button>
              if you need the boards elsewhere.
            </p>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Compliance mode blocked my project</h3>
            <p class="text-sm text-slate-300">
              That is compliance mode working: the tamper-evident audit chain failed verification,
              so the open was refused. Use
              <span class="font-medium text-slate-100">Export audit repair evidence</span> in Project
              Settings first — it preserves the raw events and failure details — then investigate
              or run the repair workflow. Turning compliance mode off skips the gate but does not
              erase the discrepancy.
            </p>
          </section>

        <!-- ── Command-Line Maintenance ──────────────────────────────── -->
        {:else if active === 'cli'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Command-Line Maintenance</h2>
          <p class="text-sm text-slate-400 mb-5">
            The GoPMgr binary doubles as a headless maintenance tool for scripts, cron jobs, and
            recovery — no GUI session required. Maintenance operations take a
            project file path (<span class="font-mono text-xs">.gopmgr</span> or
            <span class="font-mono text-xs">.pmforge</span>) as the final argument.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Operations</h3>
            <div class="overflow-x-auto">
              <table class="w-full text-sm border-collapse">
                <thead>
                  <tr class="border-b border-slate-700">
                    <th class="text-left py-1.5 pr-4 font-semibold text-slate-300 w-52">Flag</th>
                    <th class="text-left py-1.5 font-semibold text-slate-300">What it does</th>
                  </tr>
                </thead>
                <tbody class="text-slate-300">
                  {#each [
                    ['--version', 'Print the version banner and exit'],
                    ['--update', 'Check the signed update channel (no-op if none configured)'],
                    ['--check', 'Integrity check; prints ok or CORRUPT'],
                    ['--repair', 'Self-healing repair workflow with a printed action log'],
                    ['--vacuum', 'Compact the database (VACUUM)'],
                    ['--export-audit <path>', 'Write the audit log to CSV'],
                    ['--stats', 'Compact project summary: status, phase, counts'],
                    ['--schema-dump', 'Print the SQL schema (structure only, no data)'],
                    ['--export <path>', 'Render the schedule report to <path>'],
                    ['--format <fmt>', 'Export format: pdf (default), docx, odt, xlsx, csv, html, mspdi'],
                    ['--encrypt', 'AES-256-GCM encrypt the export with the --password-env password'],
                  ] as [flag, what]}
                    <tr class="border-b border-slate-800">
                      <td class="py-1.5 pr-4 font-mono text-xs text-cyan-300 whitespace-nowrap">{flag}</td>
                      <td class="py-1.5">{what}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Encrypted projects</h3>
            <p class="text-sm text-slate-300">
              Pass <span class="font-mono text-xs">--username &lt;name&gt;</span> and
              <span class="font-mono text-xs">--password-env &lt;ENVVAR&gt;</span>, where the named
              environment variable holds that account's password. The password is
              <span class="font-medium text-slate-100">never given on the command line</span>, so it
              cannot leak through the process table or shell history. Plaintext projects open
              without credentials. <span class="font-mono text-xs">--encrypt</span> reuses the same
              variable as the export password.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Examples</h3>
            <pre class="text-xs font-mono text-slate-300 bg-slate-900 border border-slate-800 rounded p-3 overflow-x-auto mb-3">{`# Integrity-check an encrypted project
export GOPMGR_PASSWORD
gopmgr --check \\
  --username alice --password-env GOPMGR_PASSWORD \\
  /path/to/project.gopmgr`}</pre>
            <pre class="text-xs font-mono text-slate-300 bg-slate-900 border border-slate-800 rounded p-3 overflow-x-auto">{`# Headless schedule export to encrypted XLSX
gopmgr --export schedule.xlsx --format xlsx --encrypt \\
  --username alice --password-env GOPMGR_PASSWORD \\
  /path/to/project.gopmgr`}</pre>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Logging</h3>
            <p class="text-sm text-slate-300">
              CLI paths log to stderr in the terminal; the GUI writes dated logs to the GoPMgr
              logs directory (App Settings &rarr; Open Logs Folder). See also
              <button onclick={() => nav('backups')} class="text-cyan-400 underline hover:text-cyan-300">Backups &amp; Data Safety</button>.
            </p>
          </section>

        {/if}
