<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import Spinner from '../Spinner.svelte';

  let types = $state<CostType[]>([]);
  let entries = $state<CostEntry[]>([]);
  let reserves = $state<CostReserve[]>([]);
  let summary = $state<CostSummary | null>(null);
  let error = $state('');
  let saving = $state(false);
  let typeID = $state('');
  let kind = $state<CostEntry['kind']>('planned');
  let amount = $state('');
  let description = $state('');
  let costDate = $state(new Date().toISOString().slice(0, 10));
  let reserveKind = $state<CostReserve['kind']>('contingency');
  let reserveAmount = $state('');
  let reserveDescription = $state('');

  async function load() {
    error = '';
    try {
      [types, entries, reserves, summary] = await Promise.all([
        window.go.main.App.ListCostTypes(),
        window.go.main.App.ListCostEntries(),
        window.go.main.App.ListCostReserves(),
        window.go.main.App.ComputeCostSummary(),
      ]);
      if (!typeID) typeID = types[0]?.id ?? '';
    } catch (err) { error = String(err); }
  }

  async function saveReserve() {
    if (saving) return;
    saving = true; error = '';
    try {
      await window.go.main.App.SaveCostReserve({ kind: reserveKind, amount: reserveAmount, description: reserveDescription });
      reserveAmount = ''; reserveDescription = ''; await load();
    } catch (err) { error = String(err); } finally { saving = false; }
  }
  onMount(load);

  async function save() {
    if (!typeID || saving) return;
    saving = true; error = '';
    try {
      await window.go.main.App.SaveCostEntry({ id: '', cost_type_id: typeID, kind, amount, description, cost_date: costDate });
      amount = ''; description = ''; await load();
    } catch (err) { error = String(err); } finally { saving = false; }
  }
</script>

<section class="bg-slate-900 border border-slate-800 rounded-lg p-4 space-y-4" aria-labelledby="cost-control-heading">
  <div>
    <h2 id="cost-control-heading" class="text-xs font-bold uppercase tracking-widest text-cyan-400">Cost control</h2>
    <p class="text-xs text-slate-500 mt-1">Ledger entries are separate from the legacy budget estimate. Amounts use {summary?.currency_code ?? 'USD'}.</p>
  </div>
  {#if error}<p class="text-xs text-red-400 break-words" role="alert">{error}</p>{/if}
  {#if !summary}<Spinner label="Loading cost control…" class="py-2" />{:else}
    <div class="grid grid-cols-2 md:grid-cols-4 gap-2 text-xs tabular-nums">
      {#each [['Funding', summary.funding], ['Base plan', summary.planned], ['Contingency reserve', summary.contingency], ['Cost baseline', summary.cost_baseline], ['Management reserve', summary.management_reserve], ['Authorised funding', summary.authorised_funding], ['Committed', summary.commitment], ['Actual', summary.actual], ['Unallocated', summary.remaining_funding]] as [label, value]}
        <div class="bg-slate-950 rounded p-2"><div class="uppercase tracking-widest text-[10px] text-slate-500">{label}</div><div class="font-bold text-slate-100">{summary.currency_code} {value}</div></div>
      {/each}
    </div>
    <form class="grid grid-cols-1 md:grid-cols-5 gap-2" onsubmit={(event) => { event.preventDefault(); void save(); }}>
      <label class="text-xs text-slate-400">Cost type<select bind:value={typeID} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100">{#each types as type}<option value={type.id}>{type.name}</option>{/each}</select></label>
      <label class="text-xs text-slate-400">State<select bind:value={kind} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100"><option value="planned">Planned</option><option value="commitment">Commitment</option><option value="actual">Actual</option></select></label>
      <label class="text-xs text-slate-400">Amount<input bind:value={amount} inputmode="decimal" required aria-label="Amount" class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="0.00" /></label>
      <label class="text-xs text-slate-400">Date<input bind:value={costDate} type="date" required class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" /></label>
      <label class="text-xs text-slate-400">Description<input bind:value={description} required class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" /></label>
      <button disabled={saving || !typeID} class="md:col-span-5 rounded bg-cyan-700 hover:bg-cyan-600 disabled:opacity-50 p-2 text-xs font-bold text-white">{saving ? 'Saving…' : 'Add ledger entry'}</button>
    </form>
    <form class="border-t border-slate-800 pt-3 grid grid-cols-1 md:grid-cols-4 gap-2" onsubmit={(event) => { event.preventDefault(); void saveReserve(); }}>
      <div class="md:col-span-4 text-[10px] tracking-widest uppercase text-slate-500">Reserve governance</div>
      <label class="text-xs text-slate-400">Reserve<select bind:value={reserveKind} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100"><option value="contingency">Contingency — known risks</option><option value="management">Management — unknown risks</option></select></label>
      <label class="text-xs text-slate-400">Amount<input bind:value={reserveAmount} inputmode="decimal" required aria-label="Reserve amount" class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="0.00" /></label>
      <label class="text-xs text-slate-400 md:col-span-2">Basis / owner note<input bind:value={reserveDescription} required class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" /></label>
      <button disabled={saving} class="md:col-span-4 rounded border border-amber-700/70 hover:bg-amber-950/40 disabled:opacity-50 p-2 text-xs font-bold text-amber-200">{saving ? 'Saving…' : 'Set reserve balance'}</button>
    </form>
    {#if reserves.length > 0}<p class="text-[10px] text-slate-500">Reserve balances are governance buffers, not posted costs. Each balance retains its basis note.</p>{/if}
    {#if entries.length > 0}<div class="overflow-x-auto"><table class="w-full text-xs"><thead class="text-left text-slate-500"><tr><th>Date</th><th>State</th><th>Description</th><th class="text-right">Amount</th></tr></thead><tbody>{#each entries as entry (entry.id)}<tr class="border-t border-slate-800"><td>{entry.cost_date}</td><td class="capitalize">{entry.kind}</td><td>{entry.description}</td><td class="text-right tabular-nums">{summary.currency_code} {entry.amount}</td></tr>{/each}</tbody></table></div>{/if}
  {/if}
</section>
