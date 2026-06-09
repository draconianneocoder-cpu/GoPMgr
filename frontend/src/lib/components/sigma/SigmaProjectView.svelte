<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
import { onMount } from 'svelte';
import { session, goto } from '../../session.svelte';
import { showToast } from '../../toast.svelte';
import SigmaPhaseStepper from './SigmaPhaseStepper.svelte';
import SigmaParetoChart from './SigmaParetoChart.svelte';
import SigmaFishbone from './SigmaFishbone.svelte';
import SigmaSolutionMatrix from './SigmaSolutionMatrix.svelte';
import SigmaControlPlan from './SigmaControlPlan.svelte';
import SigmaSIPOC from './SigmaSIPOC.svelte';
import SigmaVoCCTQ from './SigmaVoCCTQ.svelte';
import TollgateChecklist from './TollgateChecklist.svelte';

  let project = $state<SigmaProject | null>(null);
  let charter = $state<SigmaCharter | null>(null);
  let loading = $state(true);
  let saving = $state(false);
  let status = $state('');

  // Measure phase data
  let datasetInput = $state('');
  let uslInput = $state('');
  let lslInput = $state('');
  let descResult = $state<DescriptiveResult | null>(null);
  let capResult = $state<CapabilityResult | null>(null);

  // Analyze phase: Pareto data
  let paretoCategories = $state('');
  let paretoCounts = $state('');
  let paretoItems = $state<ParetoItem[]>([]);

  // Analyze phase: Fishbone data
  let fishboneData = $state<FishboneData>({ problem_statement: '', branches: [] });

  // Tollgate
  let readiness = $state<TollgateResult | null>(null);
  let showTollgate = $state(false);

  onMount(async () => {
    await loadProject();
  });

  async function loadProject() {
    loading = true;
    try {
      const projectID = session.editingId ?? '';
      if (!projectID) {
        throw new Error('Missing Six Sigma project id');
      }
      project = await window.go.main.App.SigmaGetProject(projectID);
      charter = await window.go.main.App.SigmaGetCharter(projectID);
      fishboneData = await window.go.main.App.SigmaGetFishbone(projectID);
      await loadToolStatus();
    } catch (err: any) {
      showToast(`Failed to load project: ${err}`, 'error');
    } finally {
      loading = false;
    }
  }

  async function saveCharter() {
    if (!charter || !project) return;
    saving = true;
    status = '';
    try {
      charter.project_id = project.id;
      await window.go.main.App.SigmaSaveCharter(charter);
      status = 'Charter saved.';
      showToast('Charter saved successfully', 'success');
    } catch (err: any) {
      status = `Save failed: ${err}`;
      showToast(`Charter save failed: ${err}`, 'error');
    } finally {
      saving = false;
    }
  }

  async function loadCharter() {
    if (!project) return;
    try {
      charter = await window.go.main.App.SigmaGetCharter(project.id);
      await loadToolStatus();
    } catch (err: any) {
      showToast(`Failed to refresh charter: ${err}`, 'error');
    }
  }

  async function calculateStats() {
    if (!datasetInput.trim()) return;
    try {
      const values = datasetInput.split(/[\s,]+/).map(Number).filter(n => !isNaN(n));
      if (values.length < 2) {
        showToast('Enter at least 2 numeric values', 'error');
        return;
      }
      descResult = await window.go.main.App.SigmaCalculateDescriptive(values);

      const usl = parseFloat(uslInput);
      const lsl = parseFloat(lslInput);
      if (!isNaN(usl) && !isNaN(lsl) && usl > lsl) {
        capResult = await window.go.main.App.SigmaCalculateCapability(values, usl, lsl);
      } else {
        capResult = null;
      }
      showToast('Statistics calculated', 'success');
    } catch (err: any) {
      showToast(`Calculation failed: ${err}`, 'error');
    }
  }

  async function handleDataImport(event: Event) {
    const target = event.target as HTMLInputElement;
    const file = target.files?.[0];
    if (!file) return;

    try {
      let values: number[] = [];
      const ext = file.name.split('.').pop()?.toLowerCase();

      if (ext === 'xlsx' || ext === 'xls') {
        const XLSX = await import('xlsx');
        const buffer = await file.arrayBuffer();
        const workbook = XLSX.read(buffer, { type: 'array' });
        const firstSheet = workbook.Sheets[workbook.SheetNames[0]];
        const jsonData = XLSX.utils.sheet_to_json(firstSheet, { header: 1 }) as unknown[][];

        for (const row of jsonData) {
          if (Array.isArray(row)) {
            for (const cell of row) {
              const num = typeof cell === 'number' ? cell : parseFloat(String(cell));
              if (!isNaN(num)) {
                values.push(num);
              }
            }
          }
        }
      } else {
        const text = await file.text();
        const lines = text.split(/\r?\n/).filter(line => line.trim());

        let delimiter = ',';
        if (text.includes('\t')) {
          delimiter = '\t';
        } else if (text.includes(';')) {
          delimiter = ';';
        }

        for (const line of lines) {
          const parts = line.split(delimiter).map(s => s.trim());
          for (const part of parts) {
            const num = parseFloat(part);
            if (!isNaN(num)) {
              values.push(num);
            }
          }
        }
      }

      if (values.length < 2) {
        showToast('No valid numeric data found in file', 'error');
        return;
      }

      datasetInput = values.join(', ');
      showToast(`Imported ${values.length} values from ${file.name}`, 'success');

      target.value = '';
    } catch (err: any) {
      showToast(`Failed to import file: ${err}`, 'error');
    }
  }

  async function calculatePareto() {
    try {
      const cats = paretoCategories.split(',').map(s => s.trim()).filter(Boolean);
      const counts = paretoCounts.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n));
      if (cats.length !== counts.length || cats.length === 0) {
        showToast('Categories and counts must match and not be empty', 'error');
        return;
      }
      paretoItems = await window.go.main.App.SigmaCalculatePareto(cats, counts);
      showToast('Pareto chart calculated', 'success');
    } catch (err: any) {
      showToast(`Pareto failed: ${err}`, 'error');
    }
  }

  async function checkTollgate() {
    if (!project) return;
    try {
      readiness = await window.go.main.App.SigmaCheckReadiness(project.id, project.phase);
      showTollgate = true;
    } catch (err: any) {
      showToast(`Readiness check failed: ${err}`, 'error');
    }
  }

  async function advancePhase() {
    if (!project) return;
    try {
      const res = await window.go.main.App.SigmaCheckReadiness(project.id, project.phase);
      if (!res.can_advance) {
        showToast(`Cannot advance: ${res.missing_list}`, 'error');
        readiness = res;
        showTollgate = true;
        return;
      }
      const phases = ['define', 'measure', 'analyze', 'improve', 'control'];
      const idx = phases.indexOf(project.phase);
      if (idx < phases.length - 1) {
        await window.go.main.App.SigmaAdvancePhase(project.id, phases[idx + 1]);
        showToast(`Advanced to ${phases[idx + 1]} phase`, 'success');
        await loadProject();
      }
    } catch (err: any) {
      showToast(`Advance failed: ${err}`, 'error');
    }
  }

  let phaseTools = $state<PhaseTools>({ phase: '', tools: [] });

  async function loadToolStatus() {
    if (!project) return;
    try {
      phaseTools = await window.go.main.App.SigmaGetToolStatus(project.id, project.phase);
    } catch (err: any) {
      console.error('Failed to load tool status:', err);
    }
  }

  async function exportProjectReport() {
    if (!project) return;
    try {
      const path = await window.go.main.App.SigmaExportProjectReport(project.id);
      showToast(`Report exported: ${path}`, 'success');
    } catch (err: any) {
      showToast(`Export failed: ${err}`, 'error');
    }
  }

</script>

<div class="min-h-screen bg-slate-950 text-slate-200">
  <header class="border-b border-slate-800 px-6 py-3 flex items-center justify-between">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('sigma_dashboard')} class="text-xs text-slate-400 hover:text-cyan-400">
        &larr; Sigma Home
      </button>
      <h1 class="text-sm font-bold tracking-widest uppercase text-white">
        {project?.title ?? 'Loading...'}
      </h1>
      {#if project}
        <span class="text-xs text-slate-500">{project.belt_level.toUpperCase()} · {project.phase.toUpperCase()}</span>
      {/if}
    </div>
    <button
      onclick={exportProjectReport}
      class="text-xs bg-emerald-600 hover:bg-emerald-500 text-white font-bold uppercase px-3 py-1.5 rounded"
    >
      Export Report
    </button>
  </header>

  {#if project}
    <SigmaPhaseStepper currentPhase={project.phase} projectId={project.id} />
  {/if}

  <main class="p-6 max-w-5xl mx-auto space-y-6">
    {#if status}
      <p class="text-xs {status.includes('failed') ? 'text-red-400' : 'text-cyan-400'}">{status}</p>
    {/if}

    {#if loading}
      <p class="text-sm text-slate-500">Loading project data...</p>
    {:else if project}
      <!-- Phase-specific tools -->
      <section class="grid grid-cols-1 md:grid-cols-3 gap-4">
        {#each phaseTools.tools as tool (tool.name)}
          <div class="bg-slate-900 border border-slate-800 rounded-lg p-4 flex items-center gap-3">
            <span class="text-2xl">{tool.icon}</span>
            <div class="flex-1">
              <div class="text-sm font-medium text-white">{tool.name}</div>
              <div class="text-[10px] uppercase tracking-wider {tool.status === 'completed' ? 'text-emerald-400' : tool.status === 'active' ? 'text-amber-400' : 'text-slate-400'}">
                {tool.status === 'completed' ? '✓ Completed' : tool.status === 'active' ? '● In Progress' : '○ Not Started'}
              </div>
            </div>
          </div>
        {/each}
      </section>

      <!-- Define Phase: Charter Builder -->
      {#if project.phase === 'define' && charter}
        <section class="bg-slate-900 border border-slate-800 rounded-lg p-6">
          <h2 class="text-sm font-bold uppercase tracking-widest text-cyan-400 mb-4">Project Charter</h2>

          <label class="block mb-4">
            <span class="text-xs text-slate-500 uppercase">Problem Statement</span>
            <textarea
              bind:value={charter.problem_statement}
              rows="3"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-3 rounded text-sm focus:border-cyan-500 outline-none"
              placeholder="What is wrong? Where is it happening? When did it start? How large is the problem?"
            ></textarea>
          </label>

          <label class="block mb-4">
            <span class="text-xs text-slate-500 uppercase">Business Case</span>
            <textarea
              bind:value={charter.business_case}
              rows="2"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-3 rounded text-sm focus:border-cyan-500 outline-none"
              placeholder="Why does this matter to the organization?"
            ></textarea>
          </label>

          <label class="block mb-4">
            <span class="text-xs text-slate-500 uppercase">Goal Statement</span>
            <input
              bind:value={charter.goal_statement}
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-3 rounded text-sm focus:border-cyan-500 outline-none"
              placeholder="Reduce X from Y to Z by [date]"
            />
          </label>

          <div class="flex justify-end gap-3 mt-6">
            <button
              onclick={() => goto('sigma_dashboard')}
              class="text-xs bg-slate-800 hover:bg-slate-700 px-4 py-2 rounded"
            >
              Cancel
            </button>
            <button
              onclick={saveCharter}
              disabled={saving}
              class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-2 rounded"
            >
              {saving ? 'Saving...' : 'Save Charter'}
            </button>
          </div>
        </section>

         <section class="mt-6">
           <SigmaSIPOC projectID={project.id} />
         </section>

         <!-- Define Phase: Voice of Customer to CTQ Tree -->
         {#if project.phase === 'define'}
           <section class="bg-slate-900 border border-slate-800 rounded-lg p-6">
             <h2 class="text-sm font-bold uppercase tracking-widest text-cyan-400 mb-4">Voice of Customer → CTQ Tree</h2>
             <SigmaVoCCTQ projectID={project.id} onSaved={loadCharter} />
           </section>
         {/if}
       {/if}

      <!-- Measure Phase: Analytics -->
      {#if project.phase === 'measure'}
        <section class="bg-slate-900 border border-slate-800 rounded-lg p-6">
          <h2 class="text-sm font-bold uppercase tracking-widest text-cyan-400 mb-4">Descriptive Statistics & Capability</h2>

          <div class="mb-4">
            <label class="block mb-2">
              <span class="text-xs text-slate-500 uppercase">Import Data File</span>
              <div class="mt-1 flex items-center gap-3">
                <input
                  type="file"
                  accept=".csv,.tsv,.txt,.xlsx,.xls"
                  onchange={handleDataImport}
                  class="text-sm text-slate-400 file:mr-4 file:py-2 file:px-4 file:rounded file:border-0 file:text-sm file:font-semibold file:bg-cyan-600 file:text-white hover:file:bg-cyan-500"
                />
                <span class="text-xs text-slate-500">Supports CSV, TSV, Excel (.xlsx/.xls)</span>
              </div>
            </label>
          </div>

          <label class="block mb-4">
            <span class="text-xs text-slate-500 uppercase">Dataset (comma or space separated numbers)</span>
            <textarea
              bind:value={datasetInput}
              rows="3"
              class="w-full mt-1 bg-slate-950 border border-slate-800 p-3 rounded text-sm font-mono focus:border-cyan-500 outline-none"
              placeholder="12.5, 13.1, 12.8, 13.4, 12.9..."
            ></textarea>
          </label>

          <div class="grid grid-cols-2 gap-4 mb-4">
            <label class="block">
              <span class="text-xs text-slate-500 uppercase">Upper Spec Limit (USL)</span>
              <input
                type="number"
                bind:value={uslInput}
                class="w-full mt-1 bg-slate-950 border border-slate-800 p-3 rounded text-sm font-mono focus:border-cyan-500 outline-none"
              />
            </label>
            <label class="block">
              <span class="text-xs text-slate-500 uppercase">Lower Spec Limit (LSL)</span>
              <input
                type="number"
                bind:value={lslInput}
                class="w-full mt-1 bg-slate-950 border border-slate-800 p-3 rounded text-sm font-mono focus:border-cyan-500 outline-none"
              />
            </label>
          </div>

          <button
            onclick={calculateStats}
            disabled={!datasetInput.trim()}
            class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-2 rounded"
          >
            Calculate
          </button>

          {#if descResult}
            <div class="mt-6 grid grid-cols-2 md:grid-cols-3 gap-4">
              <div class="bg-slate-950 p-3 rounded border border-slate-800">
                <div class="text-[10px] text-slate-500 uppercase">Mean</div>
                <div class="text-lg font-mono text-cyan-400">{descResult.mean.toFixed(3)}</div>
              </div>
              <div class="bg-slate-950 p-3 rounded border border-slate-800">
                <div class="text-[10px] text-slate-500 uppercase">Std Dev</div>
                <div class="text-lg font-mono text-cyan-400">{descResult.std_dev.toFixed(3)}</div>
              </div>
              <div class="bg-slate-950 p-3 rounded border border-slate-800">
                <div class="text-[10px] text-slate-500 uppercase">Count</div>
                <div class="text-lg font-mono text-cyan-400">{descResult.count}</div>
              </div>
            </div>
          {/if}

          {#if capResult}
            <div class="mt-6 grid grid-cols-2 md:grid-cols-3 gap-4">
              <div class="bg-slate-950 p-3 rounded border border-slate-800">
                <div class="text-[10px] text-slate-500 uppercase">Cp</div>
                <div class="text-lg font-mono text-emerald-400">{capResult.cp.toFixed(2)}</div>
              </div>
              <div class="bg-slate-950 p-3 rounded border border-slate-800">
                <div class="text-[10px] text-slate-500 uppercase">Cpk</div>
                <div class="text-lg font-mono text-emerald-400">{capResult.cpk.toFixed(2)}</div>
              </div>
              <div class="bg-slate-950 p-3 rounded border border-slate-800">
                <div class="text-[10px] text-slate-500 uppercase">Sigma Level</div>
                <div class="text-lg font-mono text-emerald-400">{capResult.sigma_level.toFixed(1)}</div>
              </div>
              <div class="bg-slate-950 p-3 rounded border border-slate-800">
                <div class="text-[10px] text-slate-500 uppercase">DPMO</div>
                <div class="text-lg font-mono text-emerald-400">{capResult.dpmo.toFixed(0)}</div>
              </div>
            </div>
          {/if}
        </section>
      {/if}

      <!-- Analyze Phase: Pareto Chart -->
      {#if project.phase === 'analyze'}
        <section class="bg-slate-900 border border-slate-800 rounded-lg p-6">
          <h2 class="text-sm font-bold uppercase tracking-widest text-cyan-400 mb-4">Pareto Analysis</h2>

          <div class="grid grid-cols-2 gap-4 mb-4">
            <label class="block">
              <span class="text-xs text-slate-500 uppercase">Categories (comma separated)</span>
              <input
                bind:value={paretoCategories}
                class="w-full mt-1 bg-slate-950 border border-slate-800 p-3 rounded text-sm font-mono focus:border-cyan-500 outline-none"
                placeholder="Defect A, Defect B, Defect C"
              />
            </label>
            <label class="block">
              <span class="text-xs text-slate-500 uppercase">Counts (comma separated)</span>
              <input
                bind:value={paretoCounts}
                class="w-full mt-1 bg-slate-950 border border-slate-800 p-3 rounded text-sm font-mono focus:border-cyan-500 outline-none"
                placeholder="45, 22, 12"
              />
            </label>
          </div>

          <button
            onclick={calculatePareto}
            disabled={!paretoCategories || !paretoCounts}
            class="text-xs bg-cyan-600 hover:bg-cyan-500 disabled:opacity-50 text-white font-bold uppercase px-4 py-2 rounded"
          >
            Generate Pareto
          </button>

          {#if paretoItems.length > 0}
            <div class="mt-6">
              <SigmaParetoChart items={paretoItems} />
            </div>
          {/if}
        </section>

        <section class="mt-6">
          <SigmaFishbone bind:data={fishboneData} projectId={project.id} />
        </section>
      {/if}

      <!-- Improve Phase: Solution Matrix -->
      {#if project.phase === 'improve'}
        <section class="bg-slate-900 border border-slate-800 rounded-lg p-6">
          <h2 class="text-sm font-bold uppercase tracking-widest text-cyan-400 mb-4">Solution Selection Matrix</h2>
          <SigmaSolutionMatrix projectID={project.id} />
        </section>
      {/if}

      <!-- Control Phase: Control Plan -->
      {#if project.phase === 'control'}
        <section class="bg-slate-900 border border-slate-800 rounded-lg p-6">
          <h2 class="text-sm font-bold uppercase tracking-widest text-cyan-400 mb-4">Control Plan</h2>
          <SigmaControlPlan projectID={project.id} />
        </section>
      {/if}

      <!-- Tollgate Readiness -->
      <section class="bg-slate-900 border border-slate-800 rounded-lg p-6">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-sm font-bold uppercase tracking-widest text-amber-400">Tollgate Readiness</h2>
          <div class="flex gap-2">
            <button
              onclick={checkTollgate}
              class="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1.5 rounded"
            >
              Check Readiness
            </button>
            <button
              onclick={advancePhase}
              class="text-xs bg-emerald-600 hover:bg-emerald-500 px-3 py-1.5 rounded font-bold"
            >
              Advance Phase →
            </button>
          </div>
        </div>

        {#if showTollgate && readiness}
          <TollgateChecklist
            score={readiness.score}
            canAdvance={readiness.can_advance}
            checks={readiness.checks}
            missingList={readiness.missing_list}
          />
        {/if}
      </section>
    {/if}
  </main>
</div>
