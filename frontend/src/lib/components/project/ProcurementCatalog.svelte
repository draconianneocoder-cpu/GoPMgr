<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '../../session.svelte';
  import Button from '../Button.svelte';
  import Input from '../Input.svelte';
  import Textarea from '../Textarea.svelte';

  let vendors = $state<CatalogVendor[]>([]);
  let items = $state<CatalogItem[]>([]);
  let query = $state('');
  let includeArchived = $state(false);
  let error = $state('');
  let saving = $state(false);
  let loading = $state(true);
  let vendor = $state<CatalogVendor | null>(null);
  let item = $state<CatalogItem | null>(null);

  const blankVendor = (): CatalogVendor => ({ id: '', version: 0, name: '', address: '', phone: '', fax: '', email: '', primary_contact: '', notes: '', archived: false, created_at: '', updated_at: '' });
  const blankItem = (): CatalogItem => ({ id: '', version: 0, name: '', sku: '', kind: '', default_unit: '', description: '', archived: false, created_at: '', updated_at: '' });

  async function refresh() {
    loading = true; error = '';
    try {
      [vendors, items] = await Promise.all([
        window.go.main.App.ListCatalogVendors(query, includeArchived),
        window.go.main.App.ListCatalogItems(query, includeArchived),
      ]);
    } catch (err) { error = String(err); }
    finally { loading = false; }
  }
  onMount(() => { void refresh(); });

  async function saveVendor() {
    if (!vendor || !vendor.name.trim() || saving) return;
    saving = true; error = '';
    try { await window.go.main.App.SaveCatalogVendor(vendor); vendor = null; await refresh(); }
    catch (err) { error = String(err); } finally { saving = false; }
  }
  async function saveItem() {
    if (!item || !item.name.trim() || saving) return;
    saving = true; error = '';
    try { await window.go.main.App.SaveCatalogItem(item); item = null; await refresh(); }
    catch (err) { error = String(err); } finally { saving = false; }
  }
</script>

<main class="min-h-screen bg-slate-950 text-slate-200 p-6">
  <header class="mx-auto max-w-6xl flex items-center justify-between gap-4 border-b border-slate-800 pb-4">
    <div><h1 class="text-lg font-bold text-slate-50">Reusable suppliers &amp; items</h1><p class="mt-1 text-sm text-slate-400">Private to this signed-in user. Changes apply to future project selections; posted ledger history remains unchanged.</p></div>
    <Button variant="nav" onclick={() => goto('dashboard')}>← Dashboard</Button>
  </header>
  <section class="mx-auto max-w-6xl py-6 space-y-4">
    <div class="flex flex-wrap gap-3 items-end">
      <label class="flex-1 min-w-60 text-xs text-slate-400">Search suppliers, contacts, items, or SKU<input bind:value={query} onkeydown={(event) => event.key === 'Enter' && void refresh()} class="mt-1 block w-full rounded border border-slate-700 bg-slate-900 p-2 text-sm text-slate-100" /></label>
      <label class="flex items-center gap-2 text-xs text-slate-400"><input type="checkbox" bind:checked={includeArchived} /> Include archived</label>
      <Button variant="secondary" size="sm" onclick={() => void refresh()} disabled={loading}>Search</Button>
      <Button variant="primary" size="sm" onclick={() => { vendor = blankVendor(); item = null; }}>New supplier</Button>
      <Button variant="primary" size="sm" onclick={() => { item = blankItem(); vendor = null; }}>New item</Button>
    </div>
    {#if error}<p class="rounded border border-red-800 bg-red-950/30 p-3 text-sm text-red-200" role="alert">{error}</p>{/if}
    {#if loading}<p class="text-sm text-slate-400">Loading reusable catalog…</p>{:else}
      <div class="grid gap-4 lg:grid-cols-2">
        <section class="rounded border border-slate-800 bg-slate-900"><header class="border-b border-slate-800 px-4 py-3"><h2 class="font-semibold text-slate-50">Suppliers</h2></header>{#if vendors.length === 0}<p class="p-4 text-sm text-slate-500">No matching suppliers.</p>{:else}<ul class="divide-y divide-slate-800">{#each vendors as record (record.id)}<li class="p-4"><button class="w-full text-left" onclick={() => { vendor = { ...record }; item = null; }}><span class="font-medium text-slate-100">{record.name}</span>{#if record.archived}<span class="ml-2 text-xs text-amber-300">Archived</span>{/if}<span class="block text-xs text-slate-400">{record.primary_contact}{record.email ? ` · ${record.email}` : ''}{record.phone ? ` · ${record.phone}` : ''}</span></button></li>{/each}</ul>{/if}</section>
        <section class="rounded border border-slate-800 bg-slate-900"><header class="border-b border-slate-800 px-4 py-3"><h2 class="font-semibold text-slate-50">Items, materials &amp; services</h2></header>{#if items.length === 0}<p class="p-4 text-sm text-slate-500">No matching items.</p>{:else}<ul class="divide-y divide-slate-800">{#each items as record (record.id)}<li class="p-4"><button class="w-full text-left" onclick={() => { item = { ...record }; vendor = null; }}><span class="font-medium text-slate-100">{record.name}</span>{#if record.archived}<span class="ml-2 text-xs text-amber-300">Archived</span>{/if}<span class="block text-xs text-slate-400">{record.sku}{record.kind ? ` · ${record.kind}` : ''}{record.default_unit ? ` · ${record.default_unit}` : ''}</span></button></li>{/each}</ul>{/if}</section>
      </div>
    {/if}
  </section>
</main>

{#if vendor}
  <div class="fixed inset-0 z-40 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true"><form class="w-full max-w-2xl rounded-xl border border-slate-700 bg-slate-900" onsubmit={(event) => { event.preventDefault(); void saveVendor(); }}><header class="flex items-center justify-between border-b border-slate-800 px-5 py-3"><h2 class="font-semibold">{vendor.id ? 'Edit supplier' : 'New supplier'}</h2><button type="button" onclick={() => vendor = null}>×</button></header><div class="grid gap-3 p-5 md:grid-cols-2"><label>Name<Input bind:value={vendor.name} required /></label><label>Primary contact<Input bind:value={vendor.primary_contact} /></label><label class="md:col-span-2">Address<Textarea bind:value={vendor.address} rows={2} /></label><label>Phone<Input bind:value={vendor.phone} /></label><label>Fax<Input bind:value={vendor.fax} /></label><label>Email<Input type="email" bind:value={vendor.email} /></label><label class="flex items-end gap-2"><input type="checkbox" bind:checked={vendor.archived} /> Archive supplier</label><label class="md:col-span-2">Notes<Textarea bind:value={vendor.notes} rows={3} /></label></div><footer class="flex justify-end gap-2 border-t border-slate-800 px-5 py-3"><Button variant="secondary" size="sm" onclick={() => vendor = null}>Cancel</Button><Button variant="primary" size="sm" onclick={() => void saveVendor()} disabled={saving || !vendor.name.trim()}>{saving ? 'Saving…' : 'Save supplier'}</Button></footer></form></div>
{/if}
{#if item}
  <div class="fixed inset-0 z-40 flex items-center justify-center bg-black/60 p-4" role="dialog" aria-modal="true"><form class="w-full max-w-2xl rounded-xl border border-slate-700 bg-slate-900" onsubmit={(event) => { event.preventDefault(); void saveItem(); }}><header class="flex items-center justify-between border-b border-slate-800 px-5 py-3"><h2 class="font-semibold">{item.id ? 'Edit item' : 'New item'}</h2><button type="button" onclick={() => item = null}>×</button></header><div class="grid gap-3 p-5 md:grid-cols-2"><label>Name<Input bind:value={item.name} required /></label><label>SKU<Input bind:value={item.sku} /></label><label>Kind<Input bind:value={item.kind} placeholder="Material, product, or service" /></label><label>Default unit<Input bind:value={item.default_unit} placeholder="each, hour, bag" /></label><label class="md:col-span-2">Description<Textarea bind:value={item.description} rows={3} /></label><label class="flex items-center gap-2"><input type="checkbox" bind:checked={item.archived} /> Archive item</label></div><footer class="flex justify-end gap-2 border-t border-slate-800 px-5 py-3"><Button variant="secondary" size="sm" onclick={() => item = null}>Cancel</Button><Button variant="primary" size="sm" onclick={() => void saveItem()} disabled={saving || !item.name.trim()}>{saving ? 'Saving…' : 'Save item'}</Button></footer></form></div>
{/if}
