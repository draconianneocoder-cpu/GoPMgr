<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // StakeholderEditor: Power × Interest 2x2 plot.
  //
  // Each stakeholder has a Power (low/high) and Interest (low/high)
  // classification. The backend places them inside the appropriate
  // quadrant and assigns the canonical engagement strategy:
  //
  //   high power, high interest  → Manage Closely
  //   high power, low interest   → Keep Satisfied
  //   low  power, high interest  → Keep Informed
  //   low  power, low interest   → Monitor
  //
  // Left pane: editable stakeholder list. Right pane: SVG plot.

  import { onMount, onDestroy } from 'svelte';
  import { session, goto } from '../../session.svelte';
  import { showToast } from '../../toast.svelte';
  import { autosave } from '../../autosave.svelte';

  interface Stakeholder {
    id: string;
    name: string;
    role?: string;
    power: string;    // 'low' | 'high'
    interest: string; // 'low' | 'high'
    strategy?: string;
    note?: string;
  }
  interface StakeholderDoc {
    stakeholders: Stakeholder[];
  }
  interface PlotPoint {
    id: string;
    name: string;
    role?: string;
    power: string;
    interest: string;
    strategy: string;
    x: number;
    y: number;
  }
  interface QuadrantLabel {
    power: string;
    interest: string;
    title: string;
    strategy: string;
  }
  interface StakeholderLayout {
    points: PlotPoint[];
    quadrants: QuadrantLabel[];
  }

  let chart = $state<ChartRecord | null>(null);
  let doc = $state<StakeholderDoc>({ stakeholders: [] });
  let layout = $state<StakeholderLayout>({ points: [], quadrants: [] });
  let selectedId = $state<string | null>(null);
  let status = $state('');
  let saving = $state(false);

  // Canvas dimensions for the plot. The backend returns x/y in 0..1
  // so the canvas scale is up to us.
  const W = 560;
  const H = 480;

  let stopAutosave: (() => void) | null = null;

  onMount(async () => {
    if (!session.editingId) return;
    chart = await window.go.main.App.GetChart(session.editingId);
    try {
      const parsed = JSON.parse(chart.data) as StakeholderDoc;
      doc = { stakeholders: parsed.stakeholders ?? [] };
    } catch {
      doc = { stakeholders: [] };
    }
    await refreshLayout();
    // Register for timed auto-save now the saved doc is loaded.
    stopAutosave = autosave.register(
      () => JSON.stringify(doc),
      () => save(),
    );
  });

  async function refreshLayout() {
    if (!chart) return;
    try {
      const updated = await window.go.main.App.SaveChart({
        ...chart,
        data: JSON.stringify(doc),
      });
      chart = updated;
      const res = await window.go.main.App.LayoutChart(updated.id);
      layout = res.body as StakeholderLayout;
    } catch (err: any) {
      status = `Layout failed: ${err}`;
    }
  }

  function newID(): string {
    return 'sh_' + Math.random().toString(36).slice(2, 7);
  }

  function addStakeholder() {
    const s: Stakeholder = {
      id: newID(),
      name: 'New stakeholder',
      power: 'low',
      interest: 'low',
    };
    doc.stakeholders.push(s);
    doc.stakeholders = [...doc.stakeholders];
    selectedId = s.id;
    void refreshLayout();
  }
  function removeStakeholder(id: string) {
    const before = JSON.parse(JSON.stringify(doc)) as typeof doc;
    doc.stakeholders = doc.stakeholders.filter((s) => s.id !== id);
    if (selectedId === id) selectedId = null;
    void refreshLayout();
    showToast('Stakeholder deleted', {
      type: 'info',
      undo: () => {
        doc = before;
        void refreshLayout();
      },
    });
  }

  let selected = $derived(
    selectedId ? doc.stakeholders.find((s) => s.id === selectedId) : null,
  );

  async function save() {
    if (!chart) return;
    saving = true;
    status = '';
    try {
      const updated = await window.go.main.App.SaveChart({
        ...chart,
        data: JSON.stringify(doc),
      });
      chart = updated;
      status = `Saved at ${new Date().toLocaleTimeString()}.`;
    } catch (err: any) {
      status = `Save failed: ${err}`;
    } finally {
      saving = false;
    }
  }

  // Debounced re-layout on selected stakeholder edits.
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  $effect(() => {
    if (!selected) return;
    selected.name;
    selected.role;
    selected.power;
    selected.interest;
    selected.note;
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => void refreshLayout(), 300);
  });

  // Concurrency hardening: cancel pending debounce on unmount.
  onDestroy(() => {
    stopAutosave?.();
    if (debounceTimer) {
      clearTimeout(debounceTimer);
      debounceTimer = null;
    }
  });

  // Tone for the four quadrants — colour-coded by engagement intensity.
  function quadrantTone(power: string, interest: string): string {
    if (power === 'high' && interest === 'high') return 'fill-red-950/40';
    if (power === 'high' && interest === 'low')  return 'fill-amber-950/40';
    if (power === 'low' && interest === 'high')  return 'fill-cyan-950/40';
    return 'fill-slate-900/30';
  }
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">
        Stakeholder Analysis
      </h1>
    </div>
    <div class="flex items-center gap-2">
      <button
        onclick={addStakeholder}
        class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
      >
        + Stakeholder
      </button>
      <button
        onclick={save}
        disabled={saving}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
      >
        {saving ? 'Saving...' : 'Save'}
      </button>
    </div>
  </header>

  <div class="flex">
    <!-- Stakeholder list -->
    <aside class="w-96 border-r border-slate-800 p-4 overflow-y-auto h-[calc(100vh-50px)]">
      <h2 class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-3">
        Stakeholders
      </h2>
      {#if doc.stakeholders.length === 0}
        <p class="text-xs text-slate-500">
          No stakeholders yet. Click <strong>+ Stakeholder</strong>.
        </p>
      {:else}
        <ul class="space-y-2">
          {#each doc.stakeholders as s (s.id)}
            <li>
              <button
                onclick={() => (selectedId = s.id)}
                class="w-full text-left p-2 rounded border {selectedId === s.id ? 'border-cyan-500 bg-slate-900' : 'border-slate-800 hover:bg-slate-900'}"
              >
                <div class="font-bold text-sm">{s.name}</div>
                <div class="text-[10px] text-slate-500 uppercase tracking-widest">
                  {s.power} power · {s.interest} interest
                </div>
                {#if s.role}
                  <div class="text-xs text-slate-400">{s.role}</div>
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      {/if}

      {#if selected}
        <div class="mt-6 border-t border-slate-800 pt-4 space-y-3 text-sm">
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Name</span>
            <input
              bind:value={selected.name}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Role</span>
            <input
              bind:value={selected.role}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            />
          </label>
          <div class="grid grid-cols-2 gap-2">
            <label class="block">
              <span class="text-xs text-slate-500 uppercase">Power</span>
              <select
                bind:value={selected.power}
                class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
              >
                <option value="low">Low</option>
                <option value="high">High</option>
              </select>
            </label>
            <label class="block">
              <span class="text-xs text-slate-500 uppercase">Interest</span>
              <select
                bind:value={selected.interest}
                class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded"
              >
                <option value="low">Low</option>
                <option value="high">High</option>
              </select>
            </label>
          </div>
          <label class="block">
            <span class="text-xs text-slate-500 uppercase">Notes</span>
            <textarea
              bind:value={selected.note}
              rows="3"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
            ></textarea>
          </label>
          <button
            onclick={() => removeStakeholder(selected!.id)}
            class="text-xs text-red-400 hover:text-red-300"
          >
            Remove stakeholder
          </button>
        </div>
      {/if}
    </aside>

    <!-- Plot -->
    <main class="flex-1 p-6">
      {#if status}
        <p class="text-xs text-cyan-400 mb-2" role="status" aria-live="polite">{status}</p>
      {/if}
      <svg
        role="application"
        aria-label="Stakeholder power-interest grid"
        width={W + 80}
        height={H + 80}
        class="bg-slate-900 border border-slate-800 rounded"
      >
        <!-- Axis labels -->
        <text x={40 + W / 2} y="20" font-size="11" fill="#67e8f9" text-anchor="middle" font-weight="bold"
              style="text-transform: uppercase; letter-spacing: 2px;">
          Interest →
        </text>
        <text x="20" y={40 + H / 2} font-size="11" fill="#67e8f9" text-anchor="middle" font-weight="bold"
              transform={`rotate(-90 20 ${40 + H / 2})`}
              style="text-transform: uppercase; letter-spacing: 2px;">
          Power →
        </text>

        <g transform="translate(40, 40)">
          <!-- Quadrant backgrounds -->
          {#each layout.quadrants as q (q.power + q.interest)}
            {@const x = q.interest === 'high' ? W / 2 : 0}
            {@const y = q.power === 'high' ? 0 : H / 2}
            <rect
              x={x}
              y={y}
              width={W / 2}
              height={H / 2}
              class={quadrantTone(q.power, q.interest)}
              stroke="#334155"
              stroke-width="1"
            />
            <text x={x + 10} y={y + 18} font-size="10" fill="#94a3b8"
                  style="text-transform: uppercase; letter-spacing: 1px;">
              {q.title}
            </text>
            <text x={x + 10} y={y + 36} font-size="14" fill="#f1f5f9" font-weight="bold">
              {q.strategy}
            </text>
          {/each}

          <!-- Quadrant boundary lines emphasized -->
          <line x1={W / 2} y1="0" x2={W / 2} y2={H} stroke="#475569" stroke-width="1.5" />
          <line x1="0" y1={H / 2} x2={W} y2={H / 2} stroke="#475569" stroke-width="1.5" />

          <!-- Axis tick labels -->
          <text x="6" y={H - 6} font-size="9" fill="#64748b">low</text>
          <text x={W - 26} y={H - 6} font-size="9" fill="#64748b">high</text>
          <text x="6" y={H - 6 - H / 4} font-size="9" fill="#64748b"></text>

          <!-- Stakeholder points -->
          {#each layout.points as p (p.id)}
            <g
              transform={`translate(${p.x * W}, ${p.y * H})`}
              onclick={() => (selectedId = p.id)}
              role="button"
              tabindex="0"
              aria-label={p.name}
              aria-pressed={selectedId === p.id}
              onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  selectedId = p.id;
                }
              }}
              class="cursor-pointer"
            >
              <circle
                r="14"
                fill={selectedId === p.id ? '#0e7490' : '#1e293b'}
                stroke={selectedId === p.id ? '#22d3ee' : '#334155'}
                stroke-width="2"
              />
              <text y="4" font-size="10" fill="#f1f5f9" text-anchor="middle" font-weight="bold">
                {p.name.charAt(0).toUpperCase()}
              </text>
              <text y="32" font-size="10" fill="#cbd5e1" text-anchor="middle">
                {p.name.length > 16 ? p.name.slice(0, 15) + '…' : p.name}
              </text>
            </g>
          {/each}
        </g>
      </svg>
    </main>
  </div>
</div>
