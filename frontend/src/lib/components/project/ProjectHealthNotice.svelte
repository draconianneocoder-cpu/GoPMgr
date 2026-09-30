<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { goto, session } from '../../session.svelte';

  // Shows on the Dashboard when the check a project gets on opening (Project
  // Settings: "Check this project for damage when it opens") found damage.
  // The backend runs the check once per open and remembers the result, so
  // this asks on every visit; it never repairs anything itself.
  let check = $state<OpenProjectCheck | null>(null);

  onMount(async () => {
    try {
      check = await window.go.main.App.CheckOpenProject();
    } catch {
      // The check is advice: a failed read shows nothing rather than an
      // error on the Dashboard. Check and repair remains in Project Settings.
      check = null;
    }
  });

  function checkAndRepair() {
    session.settingsTab = 'protection';
    goto('project_settings');
  }

  async function dismiss() {
    check = null;
    try {
      await window.go.main.App.DismissOpenProjectCheck();
    } catch {
      // Hidden for this visit either way; it may show again on the next one.
    }
  }
</script>

{#if check?.checked && check.damaged && !check.dismissed}
  <section
    aria-labelledby="project-health-heading"
    class="p-4 bg-red-950/30 border border-red-800/60 rounded-lg space-y-3"
  >
    <h2 id="project-health-heading" class="text-xs font-bold uppercase tracking-widest text-red-300">
      This project may be damaged
    </h2>
    <p class="text-sm text-slate-300">
      A check when the project opened found a problem in its database. Check and repair it before
      making more changes; the damaged file is kept beside the project. You can also restore it
      from a backup.
    </p>
    <div class="flex flex-wrap items-center gap-3">
      <button
        type="button"
        onclick={checkAndRepair}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 text-white font-bold px-4 py-2 rounded"
      >Check and repair</button>
      <button type="button" onclick={dismiss} class="text-xs text-slate-400 hover:text-slate-200 underline">
        Dismiss
      </button>
    </div>
  </section>
{/if}
