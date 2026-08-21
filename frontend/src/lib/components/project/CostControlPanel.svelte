<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import Spinner from '../Spinner.svelte';
  import ConfirmDialog from '../ConfirmDialog.svelte';

  let types = $state<CostType[]>([]);
  let entries = $state<CostEntry[]>([]);
  let reserves = $state<CostReserve[]>([]);
  let summary = $state<CostSummary | null>(null);
  let classifications = $state<CostClassificationSummary | null>(null);
  let error = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let typeID = $state('');
  let kind = $state<CostEntry['kind']>('planned');
  let amount = $state('');
  let description = $state('');
  let costDate = $state(new Date().toISOString().slice(0, 10));
  let reserveKind = $state<CostReserve['kind']>('contingency');
  let reserveAmount = $state('');
  let reserveDescription = $state('');
  let baselines = $state<CostBaseline[]>([]);
  let approvalNote = $state('');
  let approvalConfirmOpen = $state(false);
  const activeTypes = $derived(types.filter((type) => type.active));
  const typesByID = $derived(new Map(types.map((type) => [type.id, type])));
	const mutationDisabled = $derived(Boolean(summary?.mutation_disabled_reason));

  async function load() {
    loading = true; error = '';
    try {
      // ListCostTypes seeds a new project's taxonomy transactionally. Finish
      // that one-time write before the concurrent read models inspect it.
      types = await window.go.main.App.ListCostTypes();
      [entries, reserves, summary, classifications, baselines] = await Promise.all([
        window.go.main.App.ListCostEntries(),
        window.go.main.App.ListCostReserves(),
        window.go.main.App.ComputeCostSummary(),
        window.go.main.App.ComputeCostClassificationSummary(),
        window.go.main.App.ListCostBaselines(),
      ]);
      if (!activeTypes.some((type) => type.id === typeID)) typeID = activeTypes[0]?.id ?? '';
    } catch (err) { error = String(err); }
    finally { loading = false; }
  }
  async function approveBaseline() {
	if (saving || mutationDisabled || !approvalNote.trim()) return;
    saving = true; error = '';
    try { await window.go.main.App.ApproveCostBaseline(approvalNote.trim()); approvalNote = ''; await load(); }
    catch (err) { error = String(err); } finally { saving = false; approvalConfirmOpen = false; }
  }

  async function saveReserve() {
	if (saving || mutationDisabled) return;
    saving = true; error = '';
    try {
      await window.go.main.App.SaveCostReserve({ kind: reserveKind, amount: reserveAmount, description: reserveDescription });
      reserveAmount = ''; reserveDescription = ''; await load();
    } catch (err) { error = String(err); } finally { saving = false; }
  }
  onMount(load);

  async function save() {
	if (!typeID || saving || mutationDisabled) return;
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
  {#if loading}<Spinner label="Loading cost control…" class="py-2" />
  {:else if error}<div class="space-y-2"><p class="text-xs text-red-400 break-words" role="alert">{error}</p><button onclick={() => void load()} class="rounded border border-red-700/70 px-3 py-2 text-xs font-bold text-red-200 hover:bg-red-950/40">Retry Cost Control</button></div>
  {:else if summary && classifications}
	{#if mutationDisabled}<p class="border-l-2 border-amber-500 bg-amber-950/20 px-3 py-2 text-xs text-amber-100" role="alert">{summary.mutation_disabled_reason}</p>{/if}
    <aside class="border-l-2 border-slate-700 bg-slate-950/50 px-3 py-2 text-xs text-slate-400" aria-label="Legacy Budget context">
      <span class="font-semibold text-slate-300">Legacy budget rollup</span>
      <span class="ml-2 tabular-nums text-slate-100">{summary.currency_code} {summary.legacy_budget}</span>
      <span class="ml-2 text-slate-500">Shown for context only; it is not included in Cost Control baseline or authorised funding.</span>
    </aside>
    <div class="grid grid-cols-2 md:grid-cols-4 gap-2 text-xs tabular-nums">
      {#each [['Base plan', summary.planned], ['Contingency reserve', summary.contingency], ['Cost baseline', summary.cost_baseline], ['Management reserve', summary.management_reserve], ['Authorised funding', summary.authorised_funding], ['Committed', summary.commitment], ['Actual', summary.actual]] as [label, value]}
        <div class="bg-slate-950 rounded p-2"><div class="uppercase tracking-widest text-[10px] text-slate-500">{label}</div><div class="font-bold text-slate-100">{summary.currency_code} {value}</div></div>
      {/each}
    </div>
    <form class="grid grid-cols-1 md:grid-cols-5 gap-2" onsubmit={(event) => { event.preventDefault(); void save(); }}>
      <label class="text-xs text-slate-400">Cost type<select disabled={mutationDisabled} bind:value={typeID} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100">{#each activeTypes as type}<option value={type.id}>{type.name}</option>{/each}</select></label>
      <label class="text-xs text-slate-400">State<select disabled={mutationDisabled} bind:value={kind} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100"><option value="planned">Planned</option><option value="commitment">Commitment</option><option value="actual">Actual</option></select></label>
      <label class="text-xs text-slate-400">Amount<input disabled={mutationDisabled} bind:value={amount} inputmode="decimal" required aria-label="Amount" class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="0.00" /></label>
      <label class="text-xs text-slate-400">Date<input disabled={mutationDisabled} bind:value={costDate} type="date" required class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" /></label>
      <label class="text-xs text-slate-400">Description<input disabled={mutationDisabled} bind:value={description} required class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" /></label>
      <button disabled={saving || mutationDisabled || !typeID} class="md:col-span-5 rounded bg-cyan-700 hover:bg-cyan-600 disabled:opacity-50 p-2 text-xs font-bold text-white">{saving ? 'Saving…' : 'Add ledger entry'}</button>
    </form>
    <form class="border-t border-slate-800 pt-3 grid grid-cols-1 md:grid-cols-4 gap-2" onsubmit={(event) => { event.preventDefault(); void saveReserve(); }}>
      <div class="md:col-span-4 text-[10px] tracking-widest uppercase text-slate-500">Reserve governance</div>
      <label class="text-xs text-slate-400">Reserve<select disabled={mutationDisabled} bind:value={reserveKind} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100"><option value="contingency">Contingency — known risks</option><option value="management">Management — unknown risks</option></select></label>
      <label class="text-xs text-slate-400">Amount<input disabled={mutationDisabled} bind:value={reserveAmount} inputmode="decimal" required aria-label="Reserve amount" class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="0.00" /></label>
      <label class="text-xs text-slate-400 md:col-span-2">Basis / owner note<input disabled={mutationDisabled} bind:value={reserveDescription} required class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" /></label>
      <button disabled={saving || mutationDisabled} class="md:col-span-4 rounded border border-amber-700/70 hover:bg-amber-950/40 disabled:opacity-50 p-2 text-xs font-bold text-amber-200">{saving ? 'Saving…' : 'Set reserve balance'}</button>
    </form>
    <section class="border-t border-slate-800 pt-3 space-y-2" aria-labelledby="baseline-governance-heading">
      <div><h3 id="baseline-governance-heading" class="text-[10px] tracking-widest uppercase text-slate-500">Baseline governance</h3><p class="text-[10px] text-slate-500">Approval is a local-account record, not role authorization or an electronic signature. It snapshots Cost Control only, never the legacy Budget panel.</p></div>
      <label class="block text-xs text-slate-400">Approval rationale<input disabled={mutationDisabled} bind:value={approvalNote} required class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="Why this Cost Control baseline is approved" /></label>
      <button onclick={() => approvalConfirmOpen = true} disabled={saving || mutationDisabled || !approvalNote.trim()} class="rounded border border-cyan-700/70 hover:bg-cyan-950/40 disabled:opacity-50 p-2 text-xs font-bold text-cyan-200">Approve immutable baseline</button>
      {#if baselines.length > 0}<div class="overflow-x-auto"><table class="w-full text-xs"><thead class="text-left text-slate-500"><tr><th>Version</th><th>Approved by</th><th>Approved at</th><th>Rationale</th><th>Cost baseline</th><th>Authorised</th></tr></thead><tbody>{#each baselines as baseline (baseline.version)}<tr class="border-t border-slate-800 align-top"><td>v{baseline.version}</td><td>{baseline.approved_by}</td><td class="whitespace-nowrap"><time datetime={baseline.approved_at}>{baseline.approved_at}</time></td><td class="min-w-48 break-words text-slate-300">{baseline.approval_note}</td><td>{baseline.currency_code} {baseline.cost_baseline}</td><td>{baseline.currency_code} {baseline.authorised_funding}</td></tr>{/each}</tbody></table></div>{/if}
    </section>
    <section class="border-t border-slate-800 pt-3 space-y-2" aria-labelledby="cost-classification-heading">
      <div><h3 id="cost-classification-heading" class="text-[10px] tracking-widest uppercase text-slate-500">Classification and reconciliation</h3><p class="text-[10px] text-slate-500">Each lens independently reconciles to the ledger totals. Do not add values across lenses.</p></div>
      <div class="overflow-x-auto"><table class="w-full min-w-[38rem] text-xs"><thead class="text-left text-slate-500"><tr><th scope="col">Lens</th><th scope="col">Classification</th><th scope="col" class="text-right">Planned</th><th scope="col" class="text-right">Committed</th><th scope="col" class="text-right">Actual</th></tr></thead><tbody>{#each [{ label: 'Attribution', rows: classifications.attribution }, { label: 'Cost behavior', rows: classifications.behavior }, { label: 'Accounting treatment', rows: classifications.treatment }] as lens}{#each lens.rows as row}<tr class="border-t border-slate-800"><th scope="row" class="font-normal text-slate-400">{lens.label}</th><td class="capitalize">{row.value.replace('_', ' ')}</td><td class="text-right tabular-nums">{summary.currency_code} {row.planned}</td><td class="text-right tabular-nums">{summary.currency_code} {row.commitment}</td><td class="text-right tabular-nums">{summary.currency_code} {row.actual}</td></tr>{/each}{/each}</tbody></table></div>
    </section>
    {#if reserves.length > 0}<p class="text-[10px] text-slate-500">Reserve balances are governance buffers, not posted costs. Each balance retains its basis note.</p>{/if}
    {#if entries.length > 0}<div class="overflow-x-auto"><table class="w-full min-w-[52rem] text-xs"><thead class="text-left text-slate-500"><tr><th scope="col">Date</th><th scope="col">State</th><th scope="col">Description</th><th scope="col">Cost type</th><th scope="col">Attribution</th><th scope="col">Behavior</th><th scope="col">Treatment</th><th scope="col" class="text-right">Amount</th></tr></thead><tbody>{#each entries as entry (entry.id)}{@const type = typesByID.get(entry.cost_type_id)}<tr class="border-t border-slate-800"><td>{entry.cost_date}</td><td class="capitalize">{entry.kind}</td><td>{entry.description}</td><td>{type?.name ?? 'Unavailable type'}</td><td class="capitalize">{type?.attribution ?? 'Unavailable'}</td><td class="capitalize">{type?.behavior ?? 'Unavailable'}</td><td class="capitalize">{type?.treatment?.replace('_', ' ') ?? 'Unavailable'}</td><td class="text-right tabular-nums">{summary.currency_code} {entry.amount}</td></tr>{/each}</tbody></table></div>{/if}
  {/if}
</section>
<ConfirmDialog open={approvalConfirmOpen} title="Approve Cost Control baseline?" message="This records an immutable local-account approval snapshot. It does not approve legacy Budget values." confirmLabel="Approve baseline" tone="caution" busy={saving} onConfirm={() => void approveBaseline()} onCancel={() => approvalConfirmOpen = false} />
