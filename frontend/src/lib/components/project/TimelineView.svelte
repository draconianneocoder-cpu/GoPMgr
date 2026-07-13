<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // TimelineView renders the project's chronological event stream
  // (timeline.Build) as a horizontal SVG strip. Holiday markers
  // come from rickar/cal via the calendar wrapper.
  //
  // The strip auto-scales: x = (date − minDate) / (maxDate − minDate).
  // Sprint ranges render as a band; point events render as a tick
  // with a label above/below the strip (alternating to reduce
  // overlap).

  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';

  let entries = $state<TimelineEntry[]>([]);
  let holidays = $state<HolidayEvent[]>([]);
  let loading = $state(true);
  let error = $state('');
  let exporting = $state(false);
  let exportStatus = $state('');
  let moveStatus = $state('');
  let movingKey = $state('');
  let timelineSvg = $state<SVGSVGElement | null>(null);

  type DragState = {
    key: string;
    kind: TimelineKind;
    sourceID: string;
    originalISO: string;
    previewISO: string;
  };
  let dragging = $state<DragState | null>(null);

  // Pixel dimensions of the SVG band; the strip itself is the inner
  // rectangle.
  const W = 1100;
  const H = 320;
  const PAD_L = 60;
  const PAD_R = 30;
  const STRIP_Y = 150;
  const STRIP_H = 24;
  const innerW = W - PAD_L - PAD_R;

  onMount(async () => {
    await loadTimeline(true);
  });

  async function loadTimeline(showLoading: boolean) {
    if (showLoading) loading = true;
    try {
      entries = (await window.go.main.App.BuildTimeline()) ?? [];
      await refreshHolidays(entries);
    } catch (err: any) {
      error = `Could not load timeline: ${err}`;
    } finally {
      if (showLoading) loading = false;
    }
  }

  async function refreshHolidays(nextEntries: TimelineEntry[]) {
    if (nextEntries.length === 0) {
      holidays = [];
      return;
    }
    const dates = nextEntries.flatMap((e) => [e.date, e.end_date].filter(Boolean) as string[]);
    const times = dates.map((d) => new Date(d).getTime());
    const minDateMS = Math.min(...times);
    const maxDateMS = Math.max(...times);
    const from = new Date(minDateMS - 30 * 86400 * 1000).toISOString().slice(0, 10);
    const to = new Date(maxDateMS + 30 * 86400 * 1000).toISOString().slice(0, 10);
    holidays = (await window.go.main.App.ListHolidays(from, to)) ?? [];
  }

  // Domain bounds. Pad 5% each side so events don't kiss the edge.
  let minMS = $derived(timelineMin(entries));
  let maxMS = $derived(timelineMax(entries));
  let span = $derived(Math.max(maxMS - minMS, 86400 * 1000));

  function timelineMin(es: TimelineEntry[]): number {
    if (es.length === 0) return Date.now() - 30 * 86400 * 1000;
    return Math.min(...es.map((e) => new Date(e.date).getTime()));
  }
  function timelineMax(es: TimelineEntry[]): number {
    if (es.length === 0) return Date.now() + 30 * 86400 * 1000;
    const ends = es.map((e) =>
      e.end_date ? new Date(e.end_date).getTime() : new Date(e.date).getTime(),
    );
    return Math.max(...ends);
  }

  function xFor(iso: string): number {
    const t = new Date(iso).getTime();
    const pad = 0.05;
    const x = ((t - minMS) / span) * (1 - 2 * pad) + pad;
    return PAD_L + x * innerW;
  }

  function isoDate(iso: string): string {
    return new Date(iso).toISOString().slice(0, 10);
  }

  function entryKey(e: TimelineEntry): string {
    return `${e.kind}:${e.source_id ?? ''}`;
  }

  function canMove(e: TimelineEntry): boolean {
    return Boolean(e.editable && e.source_id);
  }

  function entryDisplayDate(e: TimelineEntry): string {
    const drag = dragging;
    return drag && drag.key === entryKey(e) ? drag.previewISO : e.date;
  }

  function entryDisplayEndDate(e: TimelineEntry): string | undefined {
    const drag = dragging;
    if (drag && drag.kind === 'sprint_end' && drag.sourceID === e.source_id) {
      return drag.previewISO;
    }
    return e.end_date;
  }

  function isoForPointer(clientX: number): string {
    if (!timelineSvg) return new Date(minMS).toISOString().slice(0, 10);
    const rect = timelineSvg.getBoundingClientRect();
    const svgX = ((clientX - rect.left) / Math.max(rect.width, 1)) * W;
    const pad = 0.05;
    const normalized = (svgX - PAD_L) / innerW;
    const fraction = Math.max(0, Math.min(1, (normalized - pad) / (1 - 2 * pad)));
    return new Date(minMS + fraction * span).toISOString().slice(0, 10);
  }

  function addDays(dateISO: string, days: number): string {
    const date = new Date(`${dateISO}T00:00:00Z`);
    date.setUTCDate(date.getUTCDate() + days);
    return date.toISOString().slice(0, 10);
  }

  function beginDrag(e: TimelineEntry, event: PointerEvent) {
    if (!canMove(e) || !e.source_id || movingKey) return;
    event.preventDefault();
    const key = entryKey(e);
    timelineSvg?.setPointerCapture(event.pointerId);
    dragging = {
      key,
      kind: e.kind,
      sourceID: e.source_id,
      originalISO: isoDate(e.date),
      previewISO: isoForPointer(event.clientX),
    };
  }

  function updateDrag(event: PointerEvent) {
    if (!dragging) return;
    dragging = { ...dragging, previewISO: isoForPointer(event.clientX) };
  }

  function cancelDrag(event: PointerEvent) {
    if (!dragging) return;
    timelineSvg?.releasePointerCapture(event.pointerId);
    dragging = null;
  }

  async function finishDrag(event: PointerEvent) {
    const drag = dragging;
    if (!drag) return;
    timelineSvg?.releasePointerCapture(event.pointerId);
    dragging = null;
    if (drag.previewISO === drag.originalISO) return;
    await moveTimelineEntry(drag.kind, drag.sourceID, drag.previewISO, drag.key);
  }

  async function moveEntryDate(e: TimelineEntry, dateISO: string) {
    if (!canMove(e) || !e.source_id || !dateISO) return;
    await moveTimelineEntry(e.kind, e.source_id, dateISO, entryKey(e));
  }

  async function moveTimelineEntry(kind: TimelineKind, sourceID: string, dateISO: string, key: string) {
    movingKey = key;
    moveStatus = '';
    try {
      entries = (await window.go.main.App.MoveTimelineEntry(kind, sourceID, dateISO)) ?? [];
      await refreshHolidays(entries);
      moveStatus = `Saved ${dateISO}.`;
    } catch (err: any) {
      moveStatus = `Move failed: ${err}`;
      await loadTimeline(false);
    } finally {
      movingKey = '';
    }
  }

  function handleEntryKey(e: TimelineEntry, event: KeyboardEvent) {
    if (!canMove(e)) return;
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
    event.preventDefault();
    const delta = event.key === 'ArrowLeft' ? -1 : 1;
    void moveEntryDate(e, addDays(isoDate(e.date), delta));
  }

  function kindColor(kind: TimelineKind): string {
    switch (kind) {
      case 'sprint_start':   return '#22d3ee';
      case 'sprint_end':     return '#0891b2';
      case 'deployment':     return '#22c55e';
      case 'project_start':  return '#f59e0b';
      case 'project_end':    return '#ef4444';
      default:               return '#94a3b8';
    }
  }

  // Generate a small number of x-axis ticks evenly spaced across
  // the timeline span. The labels are dates in `MMM d` form.
  function ticks(): { x: number; label: string }[] {
    const out: { x: number; label: string }[] = [];
    const n = 6;
    for (let i = 0; i <= n; i++) {
      const t = minMS + (i / n) * span;
      const date = new Date(t);
      const label = date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
      out.push({ x: PAD_L + ((i / n) * (1 - 0.1) + 0.05) * innerW, label });
    }
    return out;
  }

  async function exportICS(includeHolidays: boolean) {
    exporting = true;
    exportStatus = '';
    try {
      const path = await window.go.main.App.ExportProjectICS(includeHolidays);
      exportStatus = `Exported to ${path}`;
    } catch (err: any) {
      exportStatus = `Export failed: ${err}`;
    } finally {
      exporting = false;
    }
  }

</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-slate-50">Timeline</h1>
      <span class="text-xs text-slate-500">{entries.length} events</span>
    </div>
    <div class="flex gap-2">
      <button
        onclick={() => exportICS(false)}
        disabled={exporting}
        class="text-xs bg-slate-800 hover:bg-slate-700 disabled:opacity-50 px-3 py-1 rounded"
      >
        Export iCal
      </button>
      <button
        onclick={() => exportICS(true)}
        disabled={exporting}
        class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
      >
        Export iCal + holidays
      </button>
    </div>
  </header>

  <main class="p-6">
    {#if error}
      <p class="text-xs text-red-400 mb-3" role="alert">{error}</p>
    {/if}
    {#if exportStatus}
      <p class="text-xs text-cyan-400 mb-3">{exportStatus}</p>
    {/if}
    {#if moveStatus}
      <p class="text-xs text-cyan-400 mb-3">{moveStatus}</p>
    {/if}

    {#if loading}
      <p class="text-sm text-slate-500">Loading timeline…</p>
    {:else if entries.length === 0}
      <p class="text-sm text-slate-500 text-center py-12">
        No dated events yet. Set project start/end dates, plan a sprint, or record a deployment to populate this view.
      </p>
    {:else}
      <div class="overflow-x-auto bg-slate-900 border border-slate-800 rounded p-3">
        <svg
          bind:this={timelineSvg}
          width={W}
          height={H}
          class="block"
          role="application"
          aria-label="Project timeline"
          onpointermove={updateDrag}
          onpointerup={finishDrag}
          onpointercancel={cancelDrag}
        >
          <!-- Strip background -->
          <rect x={PAD_L} y={STRIP_Y} width={innerW} height={STRIP_H}
                fill="#1e293b" stroke="#334155" stroke-width="0.5" />

          <!-- Axis ticks -->
          {#each ticks() as t (t.label + t.x)}
            <line x1={t.x} y1={STRIP_Y - 4} x2={t.x} y2={STRIP_Y + STRIP_H + 4}
                  stroke="#475569" stroke-width="0.5" />
            <text x={t.x} y={STRIP_Y + STRIP_H + 16}
                  font-size="9" fill="#94a3b8" text-anchor="middle">
              {t.label}
            </text>
          {/each}

          <!-- Holidays: faint vertical stripes -->
          {#each holidays as h (h.date)}
            {@const hx = xFor(h.date)}
            <line x1={hx} y1={STRIP_Y} x2={hx} y2={STRIP_Y + STRIP_H}
                  stroke="#fb923c" stroke-width="1" stroke-dasharray="2 2" opacity="0.7" />
          {/each}

          <!-- Sprint ranges as bands -->
          {#each entries.filter((e) => e.kind === 'sprint_start' && e.end_date) as e (e.source_id + 'band')}
            {@const x1 = xFor(entryDisplayDate(e))}
            {@const x2 = xFor(entryDisplayEndDate(e)!)}
            <rect x={x1} y={STRIP_Y + 2} width={Math.max(2, x2 - x1)} height={STRIP_H - 4}
                  fill="#0e7490" opacity="0.45" rx="2" />
          {/each}

          <!-- Point events as ticks + alternating labels -->
          {#each entries as e, i (e.source_id + e.kind + e.date)}
            {@const key = entryKey(e)}
            {@const x = xFor(entryDisplayDate(e))}
            {@const editable = canMove(e)}
            {#if editable}
              <g
                role="button"
                tabindex="0"
                aria-label={`${e.title} ${isoDate(e.date)}`}
                style="cursor: {movingKey === key ? 'progress' : 'grab'}"
                onpointerdown={(event) => beginDrag(e, event)}
                onkeydown={(event) => handleEntryKey(e, event)}
              >
                <line x1={x} y1={STRIP_Y - 8} x2={x} y2={STRIP_Y + STRIP_H + 8}
                      stroke={kindColor(e.kind)} stroke-width="2" />
                <circle cx={x} cy={STRIP_Y + STRIP_H / 2} r="5"
                        fill={kindColor(e.kind)} stroke="#0f172a" stroke-width="1" />
                <circle cx={x} cy={STRIP_Y + STRIP_H / 2} r="8"
                        fill="transparent" stroke={movingKey === key ? '#fbbf24' : '#67e8f9'}
                        stroke-width="1" opacity="0.65" />
                {#if i % 2 === 0}
                  <text x={x + 4} y={STRIP_Y - 14}
                        font-size="9" fill="#cbd5e1">{e.title}</text>
                {:else}
                  <text x={x + 4} y={STRIP_Y + STRIP_H + 30}
                        font-size="9" fill="#cbd5e1">{e.title}</text>
                {/if}
              </g>
            {:else}
              <g>
                <line x1={x} y1={STRIP_Y - 8} x2={x} y2={STRIP_Y + STRIP_H + 8}
                      stroke={kindColor(e.kind)} stroke-width="1.5" />
                <circle cx={x} cy={STRIP_Y + STRIP_H / 2} r="3.5"
                        fill={kindColor(e.kind)} stroke="#0f172a" stroke-width="1" />
                {#if i % 2 === 0}
                  <text x={x + 4} y={STRIP_Y - 14}
                        font-size="9" fill="#cbd5e1">{e.title}</text>
                {:else}
                  <text x={x + 4} y={STRIP_Y + STRIP_H + 30}
                        font-size="9" fill="#cbd5e1">{e.title}</text>
                {/if}
              </g>
              {/if}
          {/each}

          {#if dragging}
            {@const previewX = xFor(dragging.previewISO)}
            <line x1={previewX} y1={STRIP_Y - 28} x2={previewX} y2={STRIP_Y + STRIP_H + 38}
                  stroke="#fbbf24" stroke-width="1" stroke-dasharray="4 3" />
            <text x={previewX + 5} y={STRIP_Y - 30}
                  font-size="10" fill="#fbbf24">{dragging.previewISO}</text>
          {/if}
        </svg>
      </div>

      <!-- Holiday legend -->
      {#if holidays.length > 0}
        <p class="mt-3 text-[10px] text-slate-500">
          <span class="inline-block w-2 h-2 align-middle"
                style="background: repeating-linear-gradient(0deg, #fb923c 0 2px, transparent 2px 4px)"></span>
          {holidays.length} holiday{holidays.length === 1 ? '' : 's'} marked from your country calendar.
        </p>
      {/if}

      <!-- Detailed list below the strip -->
      <ul class="mt-6 divide-y divide-slate-800 border border-slate-800 rounded">
        {#each entries as e (e.source_id + e.kind + e.date)}
          {@const key = entryKey(e)}
          <li class="px-3 py-2 flex items-center gap-3">
            {#if canMove(e)}
              <input
                type="date"
                value={isoDate(e.date)}
                disabled={movingKey === key}
                aria-label={`${e.title} date`}
                class="w-32 bg-slate-950 border border-slate-800 rounded px-2 py-1 text-[10px] font-mono text-slate-300 disabled:opacity-50"
                onchange={(event) => moveEntryDate(e, (event.currentTarget as HTMLInputElement).value)}
              />
            {:else}
              <span class="text-[10px] font-mono text-slate-500 w-28">
                {isoDate(e.date)}
              </span>
            {/if}
            <span class="inline-block w-2 h-2 rounded-full" style="background:{kindColor(e.kind)}"></span>
            <span class="text-xs text-slate-200 flex-1">{e.title}</span>
            <span class="text-[10px] text-slate-500 uppercase tracking-widest">{e.kind}</span>
          </li>
        {/each}
      </ul>
    {/if}
  </main>
</div>
