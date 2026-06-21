<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // Generic editor for one DocumentField. Dispatches on the field's
  // type to render the appropriate widget. This component is the
  // single source of truth for how each FieldKind looks in the GUI;
  // every document kind (all 25) reuses it.

  import ChartPicker from './ChartPicker.svelte';

  let { field, value = $bindable() }: { field: DocumentField; value: unknown } = $props();

  // Lazy default values per FieldKind. Initialised here so the first
  // keystroke doesn't have to deal with `undefined`.
  $effect(() => {
    if (value !== undefined && value !== null) return;
    switch (field.type) {
      case 'number':       value = 0; break;
      case 'bool':         value = false; break;
      case 'string_array': value = []; break;
      case 'object_array': value = []; break;
      default:             value = '';
    }
  });

  function addStringRow() {
    (value as string[]).push('');
    value = [...(value as string[])];
  }
  function removeStringRow(i: number) {
    value = (value as string[]).filter((_, idx) => idx !== i);
  }

  function addObjectRow() {
    const blank: Record<string, unknown> = {};
    for (const sub of field.object_shape ?? []) {
      blank[sub.key] = sub.type === 'number' ? 0 : sub.type === 'bool' ? false : '';
    }
    (value as Record<string, unknown>[]).push(blank);
    value = [...(value as Record<string, unknown>[])];
  }
  function removeObjectRow(i: number) {
    value = (value as Record<string, unknown>[]).filter((_, idx) => idx !== i);
  }

  function updateBool(event: Event) {
    value = (event.currentTarget as HTMLInputElement).checked;
  }
</script>

<label class="block space-y-1">
  <span class="text-xs font-semibold text-slate-500 uppercase">
    {field.label}
    {#if field.required}<span class="text-red-400">*</span>{/if}
  </span>
  {#if field.help}
    <span class="block text-[10px] text-slate-500">{field.help}</span>
  {/if}

  {#if field.type === 'string'}
    <input
      type="text"
      bind:value
      class="w-full bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
    />
  {:else if field.type === 'text'}
    <textarea
      bind:value
      rows="4"
      class="w-full bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
    ></textarea>
  {:else if field.type === 'number'}
    <input
      type="number"
      bind:value
      class="w-full bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
    />
  {:else if field.type === 'date'}
    <input
      type="date"
      bind:value
      class="w-full bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
    />
  {:else if field.type === 'bool'}
    <input
      type="checkbox"
      checked={Boolean(value)}
      onchange={updateBool}
      class="accent-cyan-500"
    />
  {:else if field.type === 'chart_ref'}
    <ChartPicker bind:value={value as string} chartKind={field.chart_kind ?? ''} />
  {:else if field.type === 'string_array'}
    <div class="space-y-2">
      {#each (value as string[]) ?? [] as _, i}
        <div class="flex gap-2">
          <input
            type="text"
            bind:value={(value as string[])[i]}
            class="flex-1 bg-slate-900 border border-slate-800 p-2 rounded focus:border-cyan-500 outline-none"
          />
          <button
            type="button"
            onclick={() => removeStringRow(i)}
            class="text-xs text-slate-500 hover:text-red-400 px-2"
          >
            ×
          </button>
        </div>
      {/each}
      <button
        type="button"
        onclick={addStringRow}
        class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
      >
        + Add
      </button>
    </div>
  {:else if field.type === 'object_array'}
    <div class="space-y-3">
      {#each (value as Record<string, unknown>[]) ?? [] as _, i}
        <div class="border border-slate-800 rounded p-3 space-y-2">
          {#each field.object_shape ?? [] as sub (sub.key)}
            <label class="block">
              <span class="text-[10px] text-slate-500 uppercase">{sub.label}</span>
              {#if sub.type === 'text'}
                <textarea
                  bind:value={(value as Record<string, unknown>[])[i][sub.key]}
                  rows="2"
                  class="w-full bg-slate-950 border border-slate-800 p-2 text-sm rounded focus:border-cyan-500 outline-none"
                ></textarea>
              {:else if sub.type === 'number'}
                <input
                  type="number"
                  bind:value={(value as Record<string, unknown>[])[i][sub.key]}
                  class="w-full bg-slate-950 border border-slate-800 p-2 text-sm rounded focus:border-cyan-500 outline-none"
                />
              {:else if sub.type === 'date'}
                <input
                  type="date"
                  bind:value={(value as Record<string, unknown>[])[i][sub.key]}
                  class="w-full bg-slate-950 border border-slate-800 p-2 text-sm rounded focus:border-cyan-500 outline-none"
                />
              {:else}
                <input
                  type="text"
                  bind:value={(value as Record<string, unknown>[])[i][sub.key]}
                  class="w-full bg-slate-950 border border-slate-800 p-2 text-sm rounded focus:border-cyan-500 outline-none"
                />
              {/if}
            </label>
          {/each}
          <button
            type="button"
            onclick={() => removeObjectRow(i)}
            class="text-xs text-slate-500 hover:text-red-400"
          >
            Remove row
          </button>
        </div>
      {/each}
      <button
        type="button"
        onclick={addObjectRow}
        class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1 rounded"
      >
        + Add row
      </button>
    </div>
  {/if}
</label>
