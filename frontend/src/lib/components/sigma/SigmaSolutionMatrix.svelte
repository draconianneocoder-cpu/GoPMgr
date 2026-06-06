<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->

<script lang="ts">
  import { onMount } from 'svelte';
  import { showToast } from '../../toast';

  let {
    projectID,
    onSaved,
  }: {
    projectID: string;
    onSaved?: () => void | Promise<void>;
  } = $props();

  let solutions = $state<SigmaSolution[]>([]);
  let loading = $state(false);

  onMount(async () => {
    await loadSolutions();
  });

  async function loadSolutions() {
    loading = true;
    try {
      solutions = await window.go.main.App.SigmaGetSolutions(projectID);
    } catch (e) {
      showToast('Failed to load solutions', 'error');
    } finally {
      loading = false;
    }
  }

  async function saveSolutions() {
    try {
      await window.go.main.App.SigmaSaveSolutions(projectID, solutions);
      showToast('Solutions saved', 'success');
      void onSaved?.();
    } catch (e) {
      showToast('Failed to save solutions', 'error');
    }
  }

  function addSolution() {
    solutions = [
      ...solutions,
      {
        id: '',
        title: '',
        description: '',
        impact: 5,
        effort: 5,
        risk: 5,
        cost: 0,
        selected: false,
        status: 'proposed',
      },
    ];
  }

  function removeSolution(index: number) {
    solutions = solutions.filter((_, i) => i !== index);
  }

  function updateSolution(index: number, field: keyof SigmaSolution, value: unknown) {
    solutions = solutions.map((s, i) => (i === index ? { ...s, [field]: value } : s));
  }

  let prioritySolutions = $derived([...solutions]
    .map((s) => ({ ...s, score: s.impact - s.effort - s.risk }))
    .sort((a, b) => b.score - a.score));
</script>

<div class="solution-matrix">
  <div class="header">
    <h3>Solution Selection Matrix</h3>
    <div class="actions">
      <button class="btn btn-secondary" onclick={addSolution}>+ Add Solution</button>
      <button class="btn btn-primary" onclick={saveSolutions}>Save</button>
    </div>
  </div>

  {#if loading}
    <p>Loading...</p>
  {:else if solutions.length === 0}
    <div class="empty-state">
      <p>No solutions added yet. Click "Add Solution" to begin.</p>
    </div>
  {:else}
    <div class="table-wrapper">
      <table>
        <thead>
          <tr>
            <th>Solution</th>
            <th>Impact (1-10)</th>
            <th>Effort (1-10)</th>
            <th>Risk (1-10)</th>
            <th>Cost ($)</th>
            <th>Priority Score</th>
            <th>Select</th>
            <th>Status</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each solutions as solution, index}
            <tr>
              <td>
                <input
                  type="text"
                  bind:value={solution.title}
                  placeholder="Solution title"
                  oninput={() => updateSolution(index, 'title', solution.title)}
                />
              </td>
              <td>
                <input
                  type="number"
                  min="1"
                  max="10"
                  bind:value={solution.impact}
                  oninput={() => updateSolution(index, 'impact', +solution.impact)}
                />
              </td>
              <td>
                <input
                  type="number"
                  min="1"
                  max="10"
                  bind:value={solution.effort}
                  oninput={() => updateSolution(index, 'effort', +solution.effort)}
                />
              </td>
              <td>
                <input
                  type="number"
                  min="1"
                  max="10"
                  bind:value={solution.risk}
                  oninput={() => updateSolution(index, 'risk', +solution.risk)}
                />
              </td>
              <td>
                <input
                  type="number"
                  min="0"
                  step="0.01"
                  bind:value={solution.cost}
                  oninput={() => updateSolution(index, 'cost', +solution.cost)}
                />
              </td>
              <td class="score">{solution.impact - solution.effort - solution.risk}</td>
              <td>
                <input
                  type="checkbox"
                  bind:checked={solution.selected}
                  onchange={() => updateSolution(index, 'selected', solution.selected)}
                />
              </td>
              <td>
                <select bind:value={solution.status} onchange={() => updateSolution(index, 'status', solution.status)}>
                  <option value="proposed">Proposed</option>
                  <option value="pilot">Pilot</option>
                  <option value="implemented">Implemented</option>
                </select>
              </td>
              <td>
                <button class="btn btn-danger btn-sm" onclick={() => removeSolution(index)}>×</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="summary">
      <h4>Priority Ranking</h4>
      <ol>
        {#each prioritySolutions as s}
          <li>{s.title || 'Untitled'} (Score: {s.impact - s.effort - s.risk})</li>
        {/each}
      </ol>
    </div>
  {/if}
</div>

<style>
  .solution-matrix {
    padding: 1rem;
  }
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .table-wrapper {
    overflow-x: auto;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    margin-bottom: 1rem;
  }
  th, td {
    padding: 0.5rem;
    border: 1px solid var(--border-color, #ccc);
    text-align: left;
  }
  th {
    background: var(--bg-secondary, #f5f5f5);
    font-weight: 600;
  }
  input[type="text"],
  input[type="number"],
  select {
    width: 100%;
    padding: 0.25rem;
    border: 1px solid var(--border-color, #ccc);
    border-radius: 4px;
  }
  .score {
    font-weight: bold;
    text-align: center;
  }
  .summary {
    margin-top: 1rem;
    padding: 1rem;
    background: var(--bg-secondary, #f5f5f5);
    border-radius: 4px;
  }
  .empty-state {
    text-align: center;
    padding: 2rem;
    color: var(--text-muted, #666);
  }
  .btn {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-weight: 500;
  }
  .btn-primary {
    background: var(--color-primary, #007bff);
    color: white;
  }
  .btn-secondary {
    background: var(--bg-secondary, #e9ecef);
    color: var(--text-color, #333);
  }
  .btn-danger {
    background: var(--color-danger, #dc3545);
    color: white;
  }
  .btn-sm {
    padding: 0.25rem 0.5rem;
    font-size: 0.875rem;
  }
</style>
