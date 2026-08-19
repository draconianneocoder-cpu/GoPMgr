<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Shared top toolbar for the post-login, project-independent screens
  // (Portfolio, Projects list, Application settings). Centralises
  // primary navigation and the sign-out control so the screens stay
  // consistent.
  import { session, goto, requestNavigation } from '../session.svelte';
  import Logo from './Logo.svelte';
  import Button from './Button.svelte';

  let { active = 'portfolio' }: { active?: 'portfolio' | 'projects' | 'settings' | 'admin' | 'help' } = $props();

  const baseNav: { key: 'portfolio' | 'projects' | 'settings' | 'admin' | 'help'; label: string; view: typeof session.view }[] = [
    { key: 'portfolio', label: 'Portfolio', view: 'portfolio' },
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
    requestNavigation('login', null, async () => {
      try {
        await window.go.main.App.Logout();
      } catch {
        return false;
      }
      session.user = null;
      session.project = null;
      session.projectPath = null;
      return true;
    });
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
      aria-label="GoPMgr home"
    >
      <span class="flex items-center gap-2">
        <Logo variant="compact" class="h-8 w-8" />
        <span class="text-sm font-bold tracking-tight text-slate-100">GoPMgr</span>
      </span>
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
    <Button variant="nav" class="underline" onclick={logout}>
      Sign out
    </Button>
  </div>
</header>
