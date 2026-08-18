<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->

<script lang="ts">
  import { onMount } from 'svelte';
  import { showToast } from '../../toast.svelte';
  import Button from '../Button.svelte';

  let {
    projectID,
    onSaved,
  }: {
    projectID: string;
    onSaved?: () => void | Promise<void>;
  } = $props();

  function blankSIPOC(): SIPOCData {
    return {
      project_id: projectID,
      process_name: '',
      process_scope: '',
      start_trigger: '',
      end_trigger: '',
      elements: [],
    };
  }

  let sipocData = $state<SIPOCData>(blankSIPOC());
  let loading = $state(false);

  const categories = [
    { key: 'supplier', label: 'Suppliers', color: 'border-blue-500' },
    { key: 'input', label: 'Inputs', color: 'border-green-500' },
    { key: 'process', label: 'Process Steps', color: 'border-purple-500' },
    { key: 'output', label: 'Outputs', color: 'border-orange-500' },
    { key: 'customer', label: 'Customers', color: 'border-red-500' },
  ];

  onMount(async () => {
    await loadSIPOC();
  });

  async function loadSIPOC() {
    loading = true;
    try {
      const data = await window.go.main.App.SigmaGetSIPOC(projectID);
      sipocData = data;
    } catch (e) {
      showToast('Failed to load SIPOC', 'error');
    } finally {
      loading = false;
    }
  }

  async function saveSIPOC() {
    try {
      await window.go.main.App.SigmaSaveSIPOC(projectID, sipocData);
      showToast('SIPOC saved', 'success');
      void onSaved?.();
    } catch (e) {
      showToast('Failed to save SIPOC', 'error');
    }
  }

  function addElement(category: string) {
    sipocData.elements = [
      ...sipocData.elements,
      {
        id: '',
        category,
        description: '',
        owner: '',
        requirements: '',
        order: sipocData.elements.filter((e) => e.category === category).length,
      },
    ];
  }

  function removeElement(index: number) {
    sipocData.elements = sipocData.elements.filter((_, i) => i !== index);
  }

  function updateElement(index: number, field: keyof SIPOCElement, value: string) {
    sipocData.elements = sipocData.elements.map((el, i) =>
      i === index ? { ...el, [field]: value } : el
    );
  }

  function getElementsForCategory(category: string) {
    return sipocData.elements.filter((el) => el.category === category);
  }
</script>

<div class="sipoc-builder">
  <div class="header">
    <h3>SIPOC Diagram</h3>
    <div class="actions">
      <Button variant="primary" size="lg" onclick={saveSIPOC}>Save</Button>
    </div>
  </div>

  {#if loading}
    <p>Loading...</p>
  {:else}
    <div class="process-info">
      <label class="block">
        <span class="label">Process Name</span>
        <input
          type="text"
          bind:value={sipocData.process_name}
          placeholder="e.g., Order Fulfillment Process"
        />
      </label>
      <label class="block">
        <span class="label">Process Scope</span>
        <input
          type="text"
          bind:value={sipocData.process_scope}
          placeholder="e.g., From order receipt to delivery"
        />
      </label>
      <div class="grid-2">
        <label class="block">
          <span class="label">Start Trigger</span>
          <input
            type="text"
            bind:value={sipocData.start_trigger}
            placeholder="e.g., Customer places order"
          />
        </label>
        <label class="block">
          <span class="label">End Trigger</span>
          <input
            type="text"
            bind:value={sipocData.end_trigger}
            placeholder="e.g., Customer receives product"
          />
        </label>
      </div>
    </div>

    <div class="sipoc-columns">
      {#each categories as cat}
        <div class="sipoc-column border-t-4 {cat.color}">
          <div class="column-header">
            <h4>{cat.label}</h4>
            <Button variant="secondary" size="sm" onclick={() => addElement(cat.key)}>+</Button>
          </div>
          <div class="column-content">
            {#each getElementsForCategory(cat.key) as element, index (element.id || index)}
              <div class="element-card">
                <textarea
                  bind:value={element.description}
                  placeholder="Description"
                  rows="2"
                  oninput={() => updateElement(sipocData.elements.indexOf(element), 'description', element.description)}
                ></textarea>
                <input
                  type="text"
                  bind:value={element.owner}
                  placeholder="Owner"
                  oninput={() => updateElement(sipocData.elements.indexOf(element), 'owner', element.owner)}
                />
                <input
                  type="text"
                  bind:value={element.requirements}
                  placeholder="Requirements"
                  oninput={() => updateElement(sipocData.elements.indexOf(element), 'requirements', element.requirements)}
                />
                <button class="btn-remove" onclick={() => removeElement(sipocData.elements.indexOf(element))}>×</button>
              </div>
            {/each}
            {#if getElementsForCategory(cat.key).length === 0}
              <p class="empty-column">No items yet. Click + to add.</p>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .sipoc-builder {
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
  .process-info {
    margin-bottom: 1.5rem;
    padding: 1rem;
    background: var(--bg-secondary, #f5f5f5);
    border-radius: 4px;
  }
  .process-info label {
    display: block;
    margin-bottom: 0.75rem;
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
  textarea {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid var(--border-color, #ccc);
    border-radius: 4px;
    font-size: 0.875rem;
  }
  .sipoc-columns {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 1rem;
  }
  .sipoc-column {
    background: var(--bg-secondary, #f5f5f5);
    border-radius: 4px;
    padding: 0.5rem;
  }
  .column-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.5rem;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid var(--border-color, #ccc);
  }
  .column-header h4 {
    font-size: 0.875rem;
    font-weight: 600;
    margin: 0;
  }
  .column-content {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .element-card {
    position: relative;
    padding: 0.5rem;
    background: white;
    border: 1px solid var(--border-color, #ddd);
    border-radius: 4px;
  }
  .element-card textarea,
  .element-card input {
    margin-bottom: 0.25rem;
    font-size: 0.75rem;
    padding: 0.25rem;
  }
  .btn-remove {
    /* #B91C1C matches Tailwind's stock red-700 (this app's danger-button
       precedent, incl. Button.svelte's danger variant) — not a themed CSS
       variable, since neither this app nor Tailwind's red-500..800 shades
       flip with the light/dark theme (see tailwind.config.js's red override,
       which only remaps 100/200/300/400/900/950). Was Bootstrap's #dc3545
       via an unset --color-danger custom property that nothing in this repo
       ever defined, i.e. the fallback was always the real value in use. */
    position: absolute;
    top: 0.25rem;
    right: 0.25rem;
    background: #b91c1c;
    color: white;
    border: none;
    border-radius: 50%;
    width: 1.25rem;
    height: 1.25rem;
    font-size: 0.75rem;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .empty-column {
    text-align: center;
    color: var(--text-muted, #999);
    font-size: 0.75rem;
    padding: 1rem 0;
  }
  @media (max-width: 1024px) {
    .sipoc-columns {
      grid-template-columns: 1fr;
    }
  }
</style>
