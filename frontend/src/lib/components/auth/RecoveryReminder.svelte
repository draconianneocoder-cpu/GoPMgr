<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { session } from '../../session.svelte';
  import RecoveryCodesPanel from './RecoveryCodesPanel.svelte';

  // Shown on the first screen after sign-in while the account has one or no
  // unused recovery codes, or codes from an older version of GoPMgr. Those
  // can't reset the password (the reset refuses them rather than replace the
  // encryption key), so this is how the user renews codes while they still
  // know their password. The embedded panel shows which case applies and
  // makes the new codes; "Not now" hides the reminder until the next sign-in.
  // Status is read on every mount, so codes renewed elsewhere clear it.
  let needed = $state(false);
  const dismissed = $derived(
    session.user !== null && session.recoveryReminderDismissedFor === session.user,
  );

  onMount(async () => {
    try {
      const status = await window.go.main.App.RecoveryCodeStatus();
      needed = status.legacy || status.unused <= 1;
    } catch {
      // The reminder is advice; App Settings still shows the codes, so a
      // failed read shows nothing rather than an error on the home screen.
      needed = false;
    }
  });

  function notNow() {
    session.recoveryReminderDismissedFor = session.user;
  }
</script>

{#if needed && !dismissed}
  <section
    aria-labelledby="recovery-reminder-heading"
    class="mb-6 p-4 bg-amber-950/30 border border-amber-700/50 rounded-lg space-y-3"
  >
    <div class="flex items-start justify-between gap-4">
      <h2 id="recovery-reminder-heading" class="text-xs font-bold uppercase tracking-widest text-amber-400">
        Keep a way back into your account
      </h2>
      <button type="button" onclick={notNow} class="text-xs text-slate-400 hover:text-slate-200 underline">
        Not now
      </button>
    </div>
    <RecoveryCodesPanel onrenewed={() => (needed = false)} />
  </section>
{/if}
