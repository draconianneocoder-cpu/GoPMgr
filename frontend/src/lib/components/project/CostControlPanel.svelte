<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import Spinner from '../Spinner.svelte';
  import ConfirmDialog from '../ConfirmDialog.svelte';

  type CatalogSupplierOption = { id: string; name: string };
  type CatalogItemOption = { id: string; name: string; sku: string; default_unit: string };

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
  let exporting = $state(false);
  let quantity = $state('');
  let unit = $state('');
  let itemName = $state('');
  let sku = $state('');
  let supplierName = $state('');
  let invoiceReference = $state('');
  let catalogItemQuery = $state('');
  let catalogSupplierQuery = $state('');
  let catalogItemOptions = $state<CatalogItemOption[]>([]);
  let catalogSupplierOptions = $state<CatalogSupplierOption[]>([]);
  let selectedCatalogItemID = $state('');
  let selectedCatalogSupplierID = $state('');
  let catalogItemNotice = $state('');
  let catalogSupplierNotice = $state('');
  let catalogItemRequest = 0;
  let catalogSupplierRequest = 0;
  let searchQuery = $state('');
  let searching = $state(false);
  let quantityAggregates = $state<CostQuantityAggregate[]>([]);
  let expandedEntryID = $state<string | null>(null);
  let attachmentsByEntry = $state<Record<string, CostEntryAttachment[]>>({});
  let attachingEntryID = $state<string | null>(null);
  let exportingAttachments = $state(false);
  const activeTypes = $derived(types.filter((type) => type.active));
  const typesByID = $derived(new Map(types.map((type) => [type.id, type])));
	const mutationDisabled = $derived(Boolean(summary?.mutation_disabled_reason));

  async function load() {
    loading = true; error = '';
    try {
      // ListCostTypes seeds a new project's taxonomy transactionally. Finish
      // that one-time write before the concurrent read models inspect it.
      types = await window.go.main.App.ListCostTypes();
      [entries, reserves, summary, classifications, baselines, quantityAggregates] = await Promise.all([
        searchQuery ? window.go.main.App.SearchCostEntries(searchQuery) : window.go.main.App.ListCostEntries(),
        window.go.main.App.ListCostReserves(),
        window.go.main.App.ComputeCostSummary(),
        window.go.main.App.ComputeCostClassificationSummary(),
        window.go.main.App.ListCostBaselines(),
        window.go.main.App.AggregateCostEntryQuantities(),
      ]);
      if (!activeTypes.some((type) => type.id === typeID)) typeID = activeTypes[0]?.id ?? '';
    } catch (err) { error = String(err); }
    finally { loading = false; }
  }

  async function runSearch() {
    if (searching) return;
    searching = true; error = '';
    try { entries = await window.go.main.App.SearchCostEntries(searchQuery); }
    catch (err) { error = String(err); } finally { searching = false; }
  }

  async function toggleAttachments(entryID: string) {
    if (expandedEntryID === entryID) { expandedEntryID = null; return; }
    expandedEntryID = entryID;
    if (attachmentsByEntry[entryID]) return;
    try { attachmentsByEntry = { ...attachmentsByEntry, [entryID]: await window.go.main.App.ListCostEntryAttachments(entryID) }; }
    catch (err) { error = String(err); }
  }

  async function attachFile(entryID: string) {
    if (attachingEntryID || mutationDisabled) return;
    attachingEntryID = entryID; error = '';
    try {
      await window.go.main.App.AttachCostEntryFile(entryID);
      attachmentsByEntry = { ...attachmentsByEntry, [entryID]: await window.go.main.App.ListCostEntryAttachments(entryID) };
    } catch (err) { if (!String(err).includes('export cancelled')) error = String(err); }
    finally { attachingEntryID = null; }
  }

  async function exportAttachmentsZip() {
    if (exportingAttachments) return;
    exportingAttachments = true; error = '';
    try { await window.go.main.App.ExportCostEntryAttachmentsZip(); }
    catch (err) { if (!String(err).includes('export cancelled')) error = String(err); }
    finally { exportingAttachments = false; }
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

  async function findCatalogItems() {
    const request = ++catalogItemRequest;
    const query = catalogItemQuery.trim();
    selectedCatalogItemID = '';
    catalogItemOptions = [];
    catalogItemNotice = '';
    if (!query) return;
    try {
      const rows = await window.go.main.App.ListCatalogItems(query, false);
      if (request !== catalogItemRequest) return;
      catalogItemOptions = rows
        .filter((row) => !row.archived)
        .map(({ id, name, sku, default_unit }) => ({ id, name, sku, default_unit }));
    } catch {
      if (request !== catalogItemRequest) return;
      catalogItemNotice = 'Catalog item lookup is unavailable. You can enter procurement detail manually.';
    }
  }

  async function findCatalogSuppliers() {
    const request = ++catalogSupplierRequest;
    const query = catalogSupplierQuery.trim();
    selectedCatalogSupplierID = '';
    catalogSupplierOptions = [];
    catalogSupplierNotice = '';
    if (!query) return;
    try {
      const rows = await window.go.main.App.ListCatalogVendors(query, false);
      if (request !== catalogSupplierRequest) return;
      catalogSupplierOptions = rows
        .filter((row) => !row.archived)
        .map(({ id, name }) => ({ id, name }));
    } catch {
      if (request !== catalogSupplierRequest) return;
      catalogSupplierNotice = 'Catalog supplier lookup is unavailable. You can enter procurement detail manually.';
    }
  }

  function copyCatalogItemDefaults() {
    if (mutationDisabled) return;
    const selected = catalogItemOptions.find((option) => option.id === selectedCatalogItemID);
    if (!selected) return;
    itemName = selected.name;
    sku = selected.sku;
    unit = selected.default_unit;
  }

  function copyCatalogSupplierName() {
    if (mutationDisabled) return;
    const selected = catalogSupplierOptions.find((option) => option.id === selectedCatalogSupplierID);
    if (!selected) return;
    supplierName = selected.name;
  }

  function clearCatalogSelections() {
    catalogItemRequest++;
    catalogSupplierRequest++;
    catalogItemQuery = '';
    catalogSupplierQuery = '';
    catalogItemOptions = [];
    catalogSupplierOptions = [];
    selectedCatalogItemID = '';
    selectedCatalogSupplierID = '';
    catalogItemNotice = '';
    catalogSupplierNotice = '';
  }

  async function save() {
	if (!typeID || saving || mutationDisabled) return;
    saving = true; error = '';
    try {
      await window.go.main.App.SaveCostEntry({ id: '', cost_type_id: typeID, kind, amount, description, cost_date: costDate, quantity, unit, item_name: itemName, sku, supplier_name: supplierName, invoice_reference: invoiceReference });
      amount = ''; description = ''; quantity = ''; unit = ''; itemName = ''; sku = ''; supplierName = ''; invoiceReference = ''; clearCatalogSelections(); await load();
    } catch (err) { error = String(err); } finally { saving = false; }
  }
  async function exportFinancialReport() {
    if (exporting) return;
    exporting = true; error = '';
    try { await window.go.main.App.ExportFinancialReportPDF(); }
    catch (err) { if (!String(err).includes('export cancelled')) error = String(err); }
    finally { exporting = false; }
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
    <div class="border-t border-slate-800 pt-3 flex flex-wrap gap-2">
      <button onclick={() => void exportFinancialReport()} disabled={exporting} class="rounded border border-cyan-700/70 hover:bg-cyan-950/40 disabled:opacity-50 px-3 py-2 text-xs font-bold text-cyan-100">{exporting ? 'Preparing financial report…' : 'Export printable financial report'}</button>
      <button onclick={() => void exportAttachmentsZip()} disabled={exportingAttachments} class="rounded border border-cyan-700/70 hover:bg-cyan-950/40 disabled:opacity-50 px-3 py-2 text-xs font-bold text-cyan-100">{exportingAttachments ? 'Preparing attachments archive…' : 'Export ledger attachments (.zip)'}</button>
      <p class="w-full text-[10px] text-slate-500">Exports this project's Legacy Budget context and Cost Control ledger separately as a printable PDF. It does not calculate a forecast or remaining funding. The attachments archive bundles every file attached to a ledger entry alongside a manifest.</p>
    </div>
    <form class="flex flex-wrap items-end gap-2" onsubmit={(event) => { event.preventDefault(); void runSearch(); }}>
      <label class="text-xs text-slate-400 flex-1 min-w-[16rem]">Search ledger<input bind:value={searchQuery} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="Item, SKU, supplier, invoice reference, or description" /></label>
      <button disabled={searching} class="rounded border border-slate-700 hover:bg-slate-800 disabled:opacity-50 px-3 py-2 text-xs font-bold text-slate-200">{searching ? 'Searching…' : 'Search'}</button>
    </form>
    <form class="grid grid-cols-1 md:grid-cols-5 gap-2" onsubmit={(event) => { event.preventDefault(); void save(); }}>
      <label class="text-xs text-slate-400">Cost type<select disabled={mutationDisabled} bind:value={typeID} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100">{#each activeTypes as type}<option value={type.id}>{type.name}</option>{/each}</select></label>
      <label class="text-xs text-slate-400">State<select disabled={mutationDisabled} bind:value={kind} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100"><option value="planned">Planned</option><option value="commitment">Commitment</option><option value="actual">Actual</option></select></label>
      <label class="text-xs text-slate-400">Amount<input disabled={mutationDisabled} bind:value={amount} inputmode="decimal" required aria-label="Amount" class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="0.00" /></label>
      <label class="text-xs text-slate-400">Date<input disabled={mutationDisabled} bind:value={costDate} type="date" required class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" /></label>
      <label class="text-xs text-slate-400">Cost item or reference<input disabled={mutationDisabled} bind:value={description} required class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="Material, invoice reference, supplier, or overhead note" /></label>
      <div class="md:col-span-5 text-[10px] tracking-widest uppercase text-slate-500 pt-1">Procurement detail (optional)</div>
      <div class="md:col-span-5 grid grid-cols-1 gap-2 rounded border border-slate-800 bg-slate-950/40 p-2 md:grid-cols-2">
        <div class="grid grid-cols-[1fr_auto] gap-2 items-end">
          <label class="text-xs text-slate-400">Find catalog item<input disabled={mutationDisabled} bind:value={catalogItemQuery} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="Item name or SKU" /></label>
          <button type="button" onclick={() => void findCatalogItems()} disabled={mutationDisabled} class="rounded border border-slate-700 hover:bg-slate-800 disabled:opacity-50 px-3 py-2 text-xs font-bold text-slate-200">Find catalog item</button>
          <label class="col-span-2 text-xs text-slate-400">Copy catalog item defaults<select disabled={mutationDisabled || catalogItemOptions.length === 0} bind:value={selectedCatalogItemID} onchange={copyCatalogItemDefaults} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100"><option value="">Select an item to copy</option>{#each catalogItemOptions as option (option.id)}<option value={option.id}>{option.name}{option.sku ? ` · ${option.sku}` : ''}{option.default_unit ? ` · ${option.default_unit}` : ''}</option>{/each}</select></label>
          <p class="col-span-2 text-[10px] text-slate-500">Copies item name, SKU, and default unit into editable project fields.</p>
          {#if catalogItemNotice}<p class="col-span-2 text-xs text-amber-200" role="status">{catalogItemNotice}</p>{/if}
        </div>
        <div class="grid grid-cols-[1fr_auto] gap-2 items-end">
          <label class="text-xs text-slate-400">Find catalog supplier<input disabled={mutationDisabled} bind:value={catalogSupplierQuery} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="Supplier name" /></label>
          <button type="button" onclick={() => void findCatalogSuppliers()} disabled={mutationDisabled} class="rounded border border-slate-700 hover:bg-slate-800 disabled:opacity-50 px-3 py-2 text-xs font-bold text-slate-200">Find catalog supplier</button>
          <label class="col-span-2 text-xs text-slate-400">Copy catalog supplier name<select disabled={mutationDisabled || catalogSupplierOptions.length === 0} bind:value={selectedCatalogSupplierID} onchange={copyCatalogSupplierName} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100"><option value="">Select a supplier to copy</option>{#each catalogSupplierOptions as option (option.id)}<option value={option.id}>{option.name}</option>{/each}</select></label>
          <p class="col-span-2 text-[10px] text-slate-500">Copies only the supplier display name. Contact details are not copied.</p>
          {#if catalogSupplierNotice}<p class="col-span-2 text-xs text-amber-200" role="status">{catalogSupplierNotice}</p>{/if}
        </div>
      </div>
      <label class="text-xs text-slate-400">Item name<input disabled={mutationDisabled} bind:value={itemName} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" /></label>
      <label class="text-xs text-slate-400">SKU<input disabled={mutationDisabled} bind:value={sku} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" /></label>
      <label class="text-xs text-slate-400">Supplier<input disabled={mutationDisabled} bind:value={supplierName} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" /></label>
      <label class="text-xs text-slate-400">Invoice reference<input disabled={mutationDisabled} bind:value={invoiceReference} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" /></label>
      <div class="grid grid-cols-2 gap-2">
        <label class="text-xs text-slate-400">Quantity<input disabled={mutationDisabled} bind:value={quantity} inputmode="decimal" class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="0.000" /></label>
        <label class="text-xs text-slate-400">Unit<input disabled={mutationDisabled} bind:value={unit} class="block mt-1 w-full bg-slate-950 border border-slate-700 rounded p-2 text-slate-100" placeholder="each, kg, hr" /></label>
      </div>
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
    {#if quantityAggregates.length > 0}
    <section class="border-t border-slate-800 pt-3 space-y-2" aria-labelledby="quantity-aggregation-heading">
      <h3 id="quantity-aggregation-heading" class="text-[10px] tracking-widest uppercase text-slate-500">Quantity by item &amp; unit</h3>
      <div class="overflow-x-auto"><table class="w-full text-xs"><thead class="text-left text-slate-500"><tr><th scope="col">Item</th><th scope="col">Unit</th><th scope="col" class="text-right">Total quantity</th><th scope="col" class="text-right">Entries</th></tr></thead><tbody>{#each quantityAggregates as agg (agg.item_name + ' ' + agg.unit)}<tr class="border-t border-slate-800"><td>{agg.item_name}</td><td>{agg.unit}</td><td class="text-right tabular-nums">{agg.total_quantity}</td><td class="text-right tabular-nums">{agg.entry_count}</td></tr>{/each}</tbody></table></div>
    </section>
    {/if}
    {#if entries.length > 0}<div class="overflow-x-auto"><table class="w-full min-w-[80rem] text-xs"><thead class="text-left text-slate-500"><tr><th scope="col">Date</th><th scope="col">State</th><th scope="col">Cost item or reference</th><th scope="col">Item</th><th scope="col">SKU</th><th scope="col">Supplier</th><th scope="col">Invoice ref</th><th scope="col" class="text-right">Quantity</th><th scope="col">Cost type</th><th scope="col">Attribution</th><th scope="col">Behavior</th><th scope="col">Treatment</th><th scope="col" class="text-right">Amount</th><th scope="col">Attachments</th></tr></thead><tbody>{#each entries as entry (entry.id)}{@const type = typesByID.get(entry.cost_type_id)}<tr class="border-t border-slate-800"><td>{entry.cost_date}</td><td class="capitalize">{entry.kind}</td><td>{entry.description}</td><td>{entry.item_name}</td><td>{entry.sku}</td><td>{entry.supplier_name}</td><td>{entry.invoice_reference}</td><td class="text-right tabular-nums">{entry.quantity}{#if entry.quantity} {entry.unit}{/if}</td><td>{type?.name ?? 'Unavailable type'}</td><td class="capitalize">{type?.attribution ?? 'Unavailable'}</td><td class="capitalize">{type?.behavior ?? 'Unavailable'}</td><td class="capitalize">{type?.treatment?.replace('_', ' ') ?? 'Unavailable'}</td><td class="text-right tabular-nums">{summary.currency_code} {entry.amount}</td><td><button type="button" onclick={() => void toggleAttachments(entry.id)} class="rounded border border-slate-700 hover:bg-slate-800 px-2 py-1 text-[10px] font-bold text-slate-200">{expandedEntryID === entry.id ? 'Hide' : 'Attachments'}</button></td></tr>{#if expandedEntryID === entry.id}<tr class="border-t border-slate-800 bg-slate-950/40"><td colspan="14" class="p-2"><div class="flex flex-wrap items-center gap-2"><button type="button" disabled={mutationDisabled || Boolean(attachingEntryID)} onclick={() => void attachFile(entry.id)} class="rounded border border-slate-700 hover:bg-slate-800 disabled:opacity-50 px-2 py-1 text-[10px] font-bold text-slate-200">{attachingEntryID === entry.id ? 'Attaching…' : 'Attach file…'}</button>{#if attachmentsByEntry[entry.id]?.length}{#each attachmentsByEntry[entry.id] as att (att.id)}<span class="rounded bg-slate-800 px-2 py-1 text-[10px] text-slate-300">{att.filename} ({Math.ceil(att.size_bytes / 1024)} KB)</span>{/each}{:else if attachmentsByEntry[entry.id]}<span class="text-[10px] text-slate-500">No attachments on this entry.</span>{/if}</div></td></tr>{/if}{/each}</tbody></table></div>{/if}
  {/if}
</section>
<ConfirmDialog open={approvalConfirmOpen} title="Approve Cost Control baseline?" message="This records an immutable local-account approval snapshot. It does not approve legacy Budget values." confirmLabel="Approve baseline" tone="caution" busy={saving} onConfirm={() => void approveBaseline()} onCancel={() => approvalConfirmOpen = false} />
