<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // MatrixEditor renders the generic m × n grid chart used for
  // requirements traceability, prioritization matrices, decision
  // matrices, etc.
  //
  // Storage shape:
  //   {
  //     "title": "",
  //     "rows_label": "Requirement",   // axis caption above row headers
  //     "cols_label": "Test Case",     // axis caption above col headers
  //     "rows": ["REQ-001", "REQ-002"],
  //     "cols": ["TC-A", "TC-B"],
  //     "cells": [["x", ""], ["", "x"]]  // [rowIdx][colIdx]
  //   }

  import { onMount } from 'svelte';
  import { session, goto } from '../../session.svelte';

  interface MatrixDoc {
    title?: string;
    rows_label?: string;
    cols_label?: string;
    rows: string[];
    cols: string[];
    cells: string[][];
  }
  interface MatrixLayout extends MatrixDoc {}

  let chart = $state<ChartRecord | null>(null);
  let doc = $state<MatrixDoc>({
    title: '',
    rows_label: '',
    cols_label: '',
    rows: [],
    cols: [],
    cells: [],
  });
  let status = $state('');
  let saving = $state(false);

  // Form inputs for add-row / add-col quick-add.
  let newRow = $state('');
  let newCol = $state('');

  onMount(async () => {
    if (!session.editingId) return;
    chart = await window.go.main.App.GetChart(session.editingId);
    try {
      const parsed = JSON.parse(chart.data) as MatrixDoc;
      doc = {
        title: parsed.title ?? '',
        rows_label: parsed.rows_label ?? '',
        cols_label: parsed.cols_label ?? '',
        rows: parsed.rows ?? [],
        cols: parsed.cols ?? [],
        cells: parsed.cells ?? [],
      };
    } catch {
      doc = { title: '', rows_label: '', cols_label: '', rows: [], cols: [], cells: [] };
    }
    normalize();
    await refreshLayout();
  });

  // Ensure cells is a strict rows × cols rectangle.
  function normalize() {
    const grid: string[][] = [];
    for (let r = 0; r < doc.rows.length; r++) {
      const row: string[] = [];
      for (let c = 0; c < doc.cols.length; c++) {
        row.push(doc.cells[r]?.[c] ?? '');
      }
      grid.push(row);
    }
    doc.cells = grid;
  }

  async function refreshLayout() {
    if (!chart) return;
    normalize();
    try {
      const updated = await window.go.main.App.SaveChart({
        ...chart,
        data: JSON.stringify(doc),
      });
      chart = updated;
      // Layout response is just the normalised shape; we already
      // hold an equivalent, so no need to re-bind.
    } catch (err: any) {
      status = `Layout failed: ${err}`;
    }
  }

  function addRow() {
    const r = newRow.trim();
    if (!r) return;
    doc.rows.push(r);
    doc.cells.push(new Array(doc.cols.length).fill(''));
    doc.rows = [...doc.rows];
    doc.cells = [...doc.cells];
    newRow = '';
    void refreshLayout();
  }
  function removeRow(idx: number) {
    doc.rows = doc.rows.filter((_, i) => i !== idx);
    doc.cells = doc.cells.filter((_, i) => i !== idx);
    void refreshLayout();
  }
  function addCol() {
    const c = newCol.trim();
    if (!c) return;
    doc.cols.push(c);
    for (const row of doc.cells) row.push('');
    doc.cols = [...doc.cols];
    doc.cells = [...doc.cells];
    newCol = '';
    void refreshLayout();
  }
  function removeCol(idx: number) {
    doc.cols = doc.cols.filter((_, i) => i !== idx);
    for (let i = 0; i < doc.cells.length; i++) {
      doc.cells[i] = doc.cells[i].filter((_, c) => c !== idx);
    }
    doc.cells = [...doc.cells];
    void refreshLayout();
  }

  function updateCell(r: number, c: number, value: string) {
    doc.cells[r][c] = value;
  }
  function renameRow(idx: number, value: string) { doc.rows[idx] = value; }
  function renameCol(idx: number, value: string) { doc.cols[idx] = value; }

  async function save() {
    if (!chart) return;
    saving = true;
    status = '';
    try {
      normalize();
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
</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Dashboard
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-white">Matrix Diagram</h1>
    </div>
    <button
      onclick={save}
      disabled={saving}
      class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-3 py-1 rounded"
    >
      {saving ? 'Saving...' : 'Save'}
    </button>
  </header>

  <main class="p-6 space-y-6">
    {#if status}
      <p class="text-xs text-cyan-400">{status}</p>
    {/if}

    <!-- Meta fields -->
    <section class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <label class="block">
        <span class="text-xs font-semibold text-slate-500 uppercase">Title</span>
        <input
          bind:value={doc.title}
          onblur={refreshLayout}
          class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <label class="block">
        <span class="text-xs font-semibold text-slate-500 uppercase">Rows axis label</span>
        <input
          bind:value={doc.rows_label}
          onblur={refreshLayout}
          placeholder="e.g. Requirement"
          class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
      <label class="block">
        <span class="text-xs font-semibold text-slate-500 uppercase">Columns axis label</span>
        <input
          bind:value={doc.cols_label}
          onblur={refreshLayout}
          placeholder="e.g. Test Case"
          class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
        />
      </label>
    </section>

    <!-- Add row / col -->
    <section class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <form onsubmit={(e) => { e.preventDefault(); addRow(); }} class="flex gap-2 items-end">
        <label class="flex-1">
          <span class="text-xs text-slate-500 uppercase">New row</span>
          <input
            bind:value={newRow}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
        <button class="bg-slate-800 hover:bg-slate-700 px-3 py-2 text-xs rounded">+ Row</button>
      </form>
      <form onsubmit={(e) => { e.preventDefault(); addCol(); }} class="flex gap-2 items-end">
        <label class="flex-1">
          <span class="text-xs text-slate-500 uppercase">New column</span>
          <input
            bind:value={newCol}
            class="w-full mt-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
        </label>
        <button class="bg-slate-800 hover:bg-slate-700 px-3 py-2 text-xs rounded">+ Column</button>
      </form>
    </section>

    {#if doc.rows.length === 0 || doc.cols.length === 0}
      <p class="text-sm text-slate-500 text-center py-8">
        Add at least one row and one column to start the matrix.
      </p>
    {:else}
      <div class="overflow-x-auto">
        <table class="border border-slate-800 text-sm">
          <thead class="bg-slate-900">
            <tr>
              <th class="p-2 border-b border-slate-800 text-[10px] text-slate-500 uppercase">
                {doc.rows_label || ''} \ {doc.cols_label || ''}
              </th>
              {#each doc.cols as col, ci (ci)}
                <th class="p-2 border-b border-l border-slate-800 min-w-[120px]">
                  <div class="flex items-center gap-1">
                    <input
                      value={col}
                      oninput={(e) => renameCol(ci, (e.target as HTMLInputElement).value)}
                      onblur={refreshLayout}
                      class="flex-1 bg-transparent text-xs px-2 py-1 rounded focus:bg-slate-800 focus:outline focus:outline-cyan-500"
                    />
                    <button
                      onclick={() => removeCol(ci)}
                      class="text-slate-500 hover:text-red-400 text-xs"
                      aria-label="Remove column"
                    >
                      ×
                    </button>
                  </div>
                </th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#each doc.rows as row, ri (ri)}
              <tr>
                <td class="p-2 border-b border-slate-800 bg-slate-900 min-w-[160px]">
                  <div class="flex items-center gap-1">
                    <input
                      value={row}
                      oninput={(e) => renameRow(ri, (e.target as HTMLInputElement).value)}
                      onblur={refreshLayout}
                      class="flex-1 bg-transparent text-xs px-2 py-1 rounded focus:bg-slate-800 focus:outline focus:outline-cyan-500"
                    />
                    <button
                      onclick={() => removeRow(ri)}
                      class="text-slate-500 hover:text-red-400 text-xs"
                      aria-label="Remove row"
                    >
                      ×
                    </button>
                  </div>
                </td>
                {#each doc.cols as _, ci (ci)}
                  <td class="p-1 border-b border-l border-slate-800">
                    <input
                      value={doc.cells[ri]?.[ci] ?? ''}
                      oninput={(e) => updateCell(ri, ci, (e.target as HTMLInputElement).value)}
                      onblur={refreshLayout}
                      class="w-full bg-transparent text-xs px-2 py-1 rounded focus:bg-slate-900 focus:outline focus:outline-cyan-500"
                    />
                  </td>
                {/each}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </main>
</div>
