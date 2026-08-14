<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import type { SectionId } from '../help-sections';

  let { active, nav }: { active: SectionId; nav: (id: SectionId) => void } = $props();
</script>

        <!-- ── Quick Start: Your First Project ──────────────────────────────── -->
        {#if active === 'quick-start'}
          <h2 class="text-xl font-bold text-slate-100 mb-1">Quick Start: Your First Project</h2>
          <p class="text-sm text-slate-400 mb-5">
            A ten-minute walkthrough from a fresh install to an exported PDF report. Every step
            names exactly what to click; no project-management experience is assumed.
          </p>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">1 · Create your account</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>Launch GoPMgr. With no accounts yet, the Create Account screen appears.</li>
              <li>Enter a username, display name, and a passphrase you can remember — it also protects your encrypted projects.</li>
              <li>As the first user you are offered the administrator role; accept it.</li>
              <li>
                GoPMgr shows your <span class="font-medium text-slate-100">eight recovery codes once</span>.
                Store them somewhere safe (password manager, printed copy) before continuing —
                they are the only way back in if you forget your passphrase. There is no cloud reset.
              </li>
            </ol>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">2 · Create a project</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>From the Portfolio screen click <span class="font-medium text-slate-100">+ New Project</span> (or press Ctrl/⌘+N).</li>
              <li>Pick an industry tile — choose <span class="font-medium text-slate-100">Software</span> for this tutorial.</li>
              <li>Pick a focus area (any), then the <span class="font-medium text-slate-100">Scrum</span> methodology.</li>
              <li>Name the project, leave the suggested starter artifacts checked, and click <span class="font-medium text-slate-100">Create Project</span>.</li>
            </ol>
            <p class="text-sm text-slate-400 mt-2">
              You land on the Project Dashboard: charts on top, documents below, budget at the side.
              The starter artifacts from the
              <button onclick={() => nav('industry-matrix')} class="text-cyan-400 underline hover:text-cyan-300">matrix</button>
              are already there.
            </p>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">3 · Build a small schedule</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>On the Dashboard open the <span class="font-medium text-slate-100">chart-tool catalog</span>, search for CPM, and choose <span class="font-medium text-slate-100">Critical Path Method Chart</span>.</li>
              <li>Click <span class="font-medium text-slate-100">+ Node</span> three times to add three activities.</li>
              <li>Click a node, then rename it and set a duration (days) in the side panel.</li>
              <li>To add a dependency: select the first node, click <span class="font-medium text-slate-100">Connect…</span>, then click the node that must follow it.</li>
              <li>Repeat until the three activities form a chain. The longest chain turns <span class="text-red-400 font-medium">red</span> — that is your critical path.</li>
              <li>Press <span class="font-medium text-slate-100">Ctrl/⌘+S</span> to save (or rely on auto-save if enabled in App Settings).</li>
            </ol>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">4 · Fill in your first document</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>Back on the Dashboard, open the seeded <span class="font-medium text-slate-100">Project Charter</span> under Documents.</li>
              <li>Fill in the purpose and objectives fields — a sentence each is enough for now.</li>
              <li>Save with Ctrl/⌘+S. The save time appears near the header, so you always know your work is on disk.</li>
            </ol>
          </section>

          <section class="mb-6">
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">5 · Export a PDF report</h3>
            <ol class="space-y-1.5 text-sm text-slate-300 list-decimal list-inside">
              <li>On the Dashboard, use the export action next to the Project Charter.</li>
              <li>In the Signature Options dialog choose <span class="font-medium text-slate-100">No digital signature</span> for now and click Export.</li>
              <li>The PDF is written to your private exports folder — the toast tells you where.</li>
            </ol>
            <p class="text-sm text-slate-400 mt-2">
              When you need tamper-evident output, come back to
              <button onclick={() => nav('export-signing')} class="text-cyan-400 underline hover:text-cyan-300">Export &amp; Digital Signing</button>
              for PAdES certificates and GnuPG signatures.
            </p>
          </section>

          <section>
            <h3 class="text-sm font-semibold text-cyan-400 uppercase tracking-wide mb-2">Where to go next</h3>
            <ul class="space-y-1.5 text-sm text-slate-300 list-disc list-inside">
              <li><button onclick={() => nav('scrum')} class="text-cyan-400 underline hover:text-cyan-300">Your methodology's guide</button> — boards, sprints, and the cadence GoPMgr sets up for you.</li>
              <li><button onclick={() => nav('charts')} class="text-cyan-400 underline hover:text-cyan-300">Charts reference</button> — all 22 chart types and when to reach for each.</li>
              <li><button onclick={() => nav('shortcuts')} class="text-cyan-400 underline hover:text-cyan-300">Keyboard Shortcuts &amp; Accessibility</button> — work faster, mouse optional.</li>
              <li><button onclick={() => nav('encryption')} class="text-cyan-400 underline hover:text-cyan-300">Database Encryption</button> — protect project files at rest.</li>
            </ul>
          </section>

        {/if}
