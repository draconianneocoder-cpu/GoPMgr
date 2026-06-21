<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->

<script lang="ts">
  import { onMount } from 'svelte';
  import { showToast } from '../../toast.svelte';

  let {
    projectID,
    onSaved,
  }: {
    projectID: string;
    onSaved?: () => void | Promise<void>;
  } = $props();

  let items = $state<SigmaControlPlanItem[]>([]);
  let loading = $state(false);

  onMount(async () => {
    await loadControlPlan();
  });

  async function loadControlPlan() {
    loading = true;
    try {
      items = await window.go.main.App.SigmaGetControlPlan(projectID);
    } catch (e) {
      showToast('Failed to load control plan', 'error');
    } finally {
      loading = false;
    }
  }

  async function saveControlPlan() {
    try {
      await window.go.main.App.SigmaSaveControlPlan(projectID, items);
      showToast('Control Plan saved', 'success');
      void onSaved?.();
    } catch (e) {
      showToast('Failed to save control plan', 'error');
    }
  }

  function addItem() {
    items = [
      ...items,
      {
        id: '',
        process_step: '',
        metric: '',
        specification: '',
        measurement_method: '',
        frequency: '',
        owner: '',
        response_plan: '',
      },
    ];
  }

  function removeItem(index: number) {
    items = items.filter((_, i) => i !== index);
  }

  function updateItem(index: number, field: keyof SigmaControlPlanItem, value: string) {
    items = items.map((item, i) => (i === index ? { ...item, [field]: value } : item));
  }
</script>

<div class="control-plan">
  <div class="header">
    <h3>Control Plan</h3>
    <div class="actions">
      <button class="btn btn-secondary" onclick={addItem}>+ Add Item</button>
      <button class="btn btn-primary" onclick={saveControlPlan}>Save</button>
    </div>
  </div>

  {#if loading}
    <p>Loading...</p>
  {:else if items.length === 0}
    <div class="empty-state">
      <p>No control plan items yet. Click "Add Item" to begin.</p>
    </div>
  {:else}
    <div class="table-wrapper">
      <table>
        <thead>
          <tr>
            <th>Process Step</th>
            <th>Metric</th>
            <th>Specification</th>
            <th>Measurement Method</th>
            <th>Frequency</th>
            <th>Owner</th>
            <th>Response Plan</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {#each items as item, index}
            <tr>
              <td>
                <input
                  type="text"
                  bind:value={item.process_step}
                  placeholder="Process step"
                  oninput={() => updateItem(index, 'process_step', item.process_step)}
                />
              </td>
              <td>
                <input
                  type="text"
                  bind:value={item.metric}
                  placeholder="Metric to monitor"
                  oninput={() => updateItem(index, 'metric', item.metric)}
                />
              </td>
              <td>
                <input
                  type="text"
                  bind:value={item.specification}
                  placeholder="Target / spec limits"
                  oninput={() => updateItem(index, 'specification', item.specification)}
                />
              </td>
              <td>
                <input
                  type="text"
                  bind:value={item.measurement_method}
                  placeholder="How to measure"
                  oninput={() => updateItem(index, 'measurement_method', item.measurement_method)}
                />
              </td>
              <td>
                <input
                  type="text"
                  bind:value={item.frequency}
                  placeholder="e.g., Every hour, Daily"
                  oninput={() => updateItem(index, 'frequency', item.frequency)}
                />
              </td>
              <td>
                <input
                  type="text"
                  bind:value={item.owner}
                  placeholder="Owner name"
                  oninput={() => updateItem(index, 'owner', item.owner)}
                />
              </td>
              <td>
                <input
                  type="text"
                  bind:value={item.response_plan}
                  placeholder="Action if out of control"
                  oninput={() => updateItem(index, 'response_plan', item.response_plan)}
                />
              </td>
              <td>
                <button class="btn btn-danger btn-sm" onclick={() => removeItem(index)}>×</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .control-plan {
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
  input[type="text"] {
    width: 100%;
    padding: 0.25rem;
    border: 1px solid var(--border-color, #ccc);
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
