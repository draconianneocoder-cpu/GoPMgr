<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Shared top toolbar for the post-login, project-independent screens
  // (Portfolio dashboard, Projects list, Application settings). Centralises
  // primary navigation and the sign-out control so the screens stay
  // consistent.
  import { session, goto } from '../session.svelte';
  import Logo from './Logo.svelte';

  let { active = 'portfolio' }: { active?: 'portfolio' | 'projects' | 'settings' | 'admin' | 'help' } = $props();

  const baseNav: { key: 'portfolio' | 'projects' | 'settings' | 'admin' | 'help'; label: string; view: typeof session.view }[] = [
    { key: 'portfolio', label: 'Dashboard', view: 'portfolio' },
    { key: 'projects', label: 'Projects', view: 'project_picker' },
    { key: 'settings', label: 'App Settings', view: 'app_settings' },
    { key: 'help', label: 'Help', view: 'help' },
  ];

  const nav = $derived(
    session.user?.is_admin
      ? [...baseNav, { key: 'admin' as const, label: 'Admin', view: 'admin_panel' as typeof session.view }]
      : baseNav
  );

  async function logout() {
    try {
      await window.go.main.App.Logout();
    } catch {
      /* ignore */
    }
    session.user = null;
    session.project = null;
    session.projectPath = null;
    goto('login');
  }
</script>

<header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between gap-4">
  <div class="flex items-center gap-6 min-w-0">
    <a
      href="#dashboard"
      onclick={(e) => {
        e.preventDefault();
        goto('portfolio');
      }}
      class="shrink-0 text-slate-100"
      aria-label="PMForge home"
    >
      <Logo class="h-6 text-slate-100" />
    </a>
    <nav class="flex items-center gap-1" aria-label="Primary">
      {#each nav as item (item.key)}
        <button
          onclick={() => goto(item.view)}
          aria-current={active === item.key ? 'page' : undefined}
          class={`text-xs font-semibold uppercase tracking-wider px-3 py-1.5 rounded ${
            active === item.key
              ? 'bg-slate-800 text-cyan-400'
              : 'text-slate-400 hover:text-cyan-400 hover:bg-slate-800/60'
          }`}
        >
          {item.label}
        </button>
      {/each}
    </nav>
  </div>
  <div class="flex items-center gap-4 shrink-0">
    <span class="text-xs text-slate-500 hidden sm:inline">
      {session.user?.display_name ?? session.user?.username}
    </span>
    <button onclick={logout} class="text-xs text-slate-400 hover:text-cyan-400 underline">
      Sign out
    </button>
  </div>
</header>
