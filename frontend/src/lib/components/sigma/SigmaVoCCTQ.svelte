<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
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

  function blankVoC(): VoCData {
    return {
      project_id: projectID,
      entries: [],
    };
  }

  let vocData = $state<VoCData>(blankVoC());
  let loading = $state(false);

  onMount(async () => {
    await loadVoC();
  });

  async function loadVoC() {
    loading = true;
    try {
      const data = await window.go.main.App.SigmaGetVoC(projectID);
      vocData = data;
    } catch (e) {
      showToast('Failed to load VoC data', 'error');
    } finally {
      loading = false;
    }
  }

  async function saveVoC() {
    try {
      await window.go.main.App.SigmaSaveVoC(projectID, vocData);
      showToast('VoC data saved', 'success');
      void onSaved?.();
    } catch (e) {
      showToast('Failed to save VoC data', 'error');
    }
  }

  function addEntry() {
    vocData.entries = [
      ...vocData.entries,
      {
        id: '',
        customer_need: '',
        ctq: '',
        lower_spec: 0,
        upper_spec: 0,
        measurement: '',
        data_collection: '',
        priority: 3,
        source: '',
      },
    ];
  }

  function removeEntry(index: number) {
    vocData.entries = vocData.entries.filter((_, i) => i !== index);
  }

  function updateEntry(index: number, field: keyof VoCEntry, value: string | number) {
    vocData.entries = vocData.entries.map((entry, i) =>
      i === index ? { ...entry, [field]: value } : entry
    );
  }

  // Convert VoC entries to CTQs for the charter
  let ctqs = $derived(vocData.entries
    .filter(entry => entry.ctq.trim() !== '' && (entry.lower_spec !== 0 || entry.upper_spec !== 0))
    .map(entry => ({
      customer_need: entry.customer_need,
      ctq: entry.ctq,
      lower_spec: entry.lower_spec,
      upper_spec: entry.upper_spec,
    })));
</script>

<div class="voc-ctq-builder">
  <div class="header">
    <h3>Voice of Customer → CTQ Tree</h3>
    <div class="actions">
      <button class="btn btn-secondary" onclick={addEntry}>+ Add Entry</button>
      <button class="btn btn-primary" onclick={saveVoC}>Save</button>
    </div>
  </div>

  {#if loading}
    <p>Loading...</p>
  {:else if vocData.entries.length === 0}
    <div class="empty-state">
      <p>No VoC entries yet. Click "Add Entry" to begin capturing customer needs.</p>
    </div>
  {:else}
    <div class="voc-entries">
      {#each vocData.entries as entry, index (entry.id || index)}
        <div class="voc-entry-card">
          <div class="entry-header">
            <h4>Entry #{index + 1}</h4>
            <button class="btn btn-sm btn-danger" onclick={() => removeEntry(index)}>×</button>
          </div>

          <div class="entry-content">
            <label class="block">
              <span class="label">Customer Need</span>
              <textarea
                bind:value={entry.customer_need}
                placeholder="What does the customer say they need or want?"
                rows="2"
                oninput={() => updateEntry(index, 'customer_need', entry.customer_need)}
              ></textarea>
            </label>

            <label class="block">
              <span class="label">CTQ (Critical to Quality)</span>
              <input
                type="text"
                bind:value={entry.ctq}
                placeholder="What must we control to satisfy this need?"
                oninput={() => updateEntry(index, 'ctq', entry.ctq)}
              />
            </label>

            <div class="grid-2">
              <label class="block">
                <span class="label">Lower Spec</span>
                <input
                  type="number"
                  bind:value={entry.lower_spec}
                  step="any"
                  oninput={() => updateEntry(index, 'lower_spec', entry.lower_spec)}
                />
              </label>
              <label class="block">
                <span class="label">Upper Spec</span>
                <input
                  type="number"
                  bind:value={entry.upper_spec}
                  step="any"
                  oninput={() => updateEntry(index, 'upper_spec', entry.upper_spec)}
                />
              </label>
            </div>

            <div class="grid-2">
              <label class="block">
                <span class="label">Measurement</span>
                <input
                  type="text"
                  bind:value={entry.measurement}
                  placeholder="How do we measure this?"
                  oninput={() => updateEntry(index, 'measurement', entry.measurement)}
                />
              </label>
              <label class="block">
                <span class="label">Data Collection Plan</span>
                <input
                  type="text"
                  bind:value={entry.data_collection}
                  placeholder="How will we collect data?"
                  oninput={() => updateEntry(index, 'data_collection', entry.data_collection)}
                />
              </label>
            </div>

            <div class="grid-2">
              <label class="block">
                <span class="label">Priority (1-5)</span>
                <input
                  type="number"
                  min="1"
                  max="5"
                  bind:value={entry.priority}
                  oninput={() => updateEntry(index, 'priority', Number(entry.priority))}
                />
              </label>
              <label class="block">
                <span class="label">Source</span>
                <input
                  type="text"
                  bind:value={entry.source}
                  placeholder="Survey, interview, complaint, etc."
                  oninput={() => updateEntry(index, 'source', entry.source)}
                />
              </label>
            </div>
          </div>
        </div>
      {/each}
    </div>

    {#if ctqs.length > 0}
      <div class="ctq-summary">
        <h4>CTQs for Charter ({ctqs.length})</h4>
        <ol>
          {#each ctqs as ctq}
            <li>
              <strong>{ctq.ctq}</strong>: {ctq.customer_need}
              [{ctq.lower_spec}, {ctq.upper_spec}]
            </li>
          {/each}
        </ol>
      </div>
    {/if}
  {/if}
</div>

<style>
  .voc-ctq-builder {
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
  .voc-entries {
    margin-bottom: 1.5rem;
  }
  .voc-entry-card {
    border: 1px solid var(--border-color, #ddd);
    border-radius: 8px;
    padding: 1rem;
    margin-bottom: 1rem;
    background: white;
  }
  .entry-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 0.75rem;
  }
  .entry-header h4 {
    margin: 0 0 0.25rem 0;
    font-size: 1rem;
    color: var(--text-color, #333);
  }
  .entry-content {
    display: grid;
    gap: 0.75rem;
  }
  .label {
    display: block;
    font-size: 0.75rem;
    text-transform: uppercase;
    color: var(--text-muted, #666);
    margin-bottom: 0.25rem;
  }
  .grid-2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }
  input[type="text"],
  input[type="number"],
  textarea {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid var(--border-color, #ccc);
    border-radius: 4px;
    font-size: 0.875rem;
  }
  input[type="text"]:focus,
  input[type="number"]:focus,
  textarea:focus {
    outline: none;
    border-color: var(--color-primary, #007bff);
    box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
  }
  .ctq-summary {
    margin-top: 1.5rem;
    padding: 1rem;
    background: var(--bg-secondary, #f5f5f5);
    border-radius: 4px;
  }
  .ctq-summary h4 {
    margin-top: 0;
    margin-bottom: 0.75rem;
    font-size: 1.125rem;
    color: var(--text-color, #333);
  }
  .ctq-summary ol {
    padding-left: 1.5rem;
  }
  .ctq-summary li {
    margin-bottom: 0.5rem;
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
