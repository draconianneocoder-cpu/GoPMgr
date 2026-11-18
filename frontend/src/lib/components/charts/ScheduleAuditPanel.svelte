<!--
SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // ScheduleAuditPanel runs the PS-02 schedule-quality audit on the chart
  // being edited and lists its findings. The audit only reads the stored
  // chart, so the panel first saves edits auto-save has not recorded yet
  // (saveIfDirty); if that fails it does not audit, because the findings
  // would describe an older revision. Severity is always written as text.
  import { desktopApi } from '../../desktop-api';
  import { normaliseAuditResult } from '../../desktop-dtos';
  import { formatNumber, t } from '../../i18n/i18n.svelte';
  import { en, type MessageKey } from '../../i18n/messages/en';
  import { localDateString } from '../../local-date';
  import Button from '../Button.svelte';

  let {
    currentChartId,
    saveIfDirty,
    selectTask,
    taskExists,
  }: {
    /** The chart the editor holds and saves, which the audit reads. */
    currentChartId: () => string | null;
    /** Saves unrecorded edits; false when the save failed. */
    saveIfDirty: () => Promise<boolean>;
    /** Selects a task on the chart; false when it is not there. */
    selectTask: (id: string) => boolean;
    /** Whether the chart still has a task with this ID. */
    taskExists: (id: string) => boolean;
  } = $props();

  let profile = $state<ScheduleAuditProfileKey>('default');
  let statusDate = $state(localDateString());
  let busy = $state(false);
  let error = $state('');
  let result = $state<ScheduleAuditResult | null>(null);
  let announcement = $state('');

  const severityKey: Record<string, MessageKey> = {
    error: 'audit.severity.error',
    warning: 'audit.severity.warning',
    info: 'audit.severity.info',
  };

  // ruleName is the catalog's name for a rule; a rule this build does not
  // know is shown by its ID alone.
  function ruleName(rule: string): string {
    const key = `audit.rule.${rule}`;
    return key in en ? t(key as MessageKey) : rule;
  }

  function percent(n: number): string {
    return formatNumber(n / 100, { style: 'percent', maximumFractionDigits: 1 });
  }

  let skipped = $derived(result ? result.rules.filter((r) => !r.evaluated) : []);
  let counts = $derived.by(() => {
    const c = { error: 0, warning: 0, info: 0 };
    for (const f of result?.findings ?? []) {
      if (f.severity === 'error') c.error++;
      else if (f.severity === 'warning') c.warning++;
      else c.info++;
    }
    return c;
  });
  let summary = $derived(
    result
      ? t('audit.summary', {
          errors: t('audit.count.errors', { count: counts.error }),
          warnings: t('audit.count.warnings', { count: counts.warning }),
          info: t('audit.count.info', { count: counts.info }),
        })
      : '',
  );

  async function run() {
    if (busy) return;
    busy = true;
    error = '';
    // A new run replaces the old result, so a failed run never leaves stale
    // findings beside its error; the status region says the audit started.
    result = null;
    announcement = t('audit.running');
    try {
      const chartId = currentChartId();
      if (!chartId || !(await saveIfDirty())) {
        error = t('audit.saveFailed');
        announcement = '';
        return;
      }
      result = normaliseAuditResult(await desktopApi.AuditSchedule(chartId, statusDate, profile));
      announcement = '';
    } catch (err: any) {
      error = t('audit.failed', { error: String(err?.message ?? err) });
      announcement = '';
    } finally {
      busy = false;
    }
  }

  function show(id: string, title: string) {
    if (selectTask(id)) announcement = t('audit.selected', { title: title || id, id });
  }
</script>

<section aria-labelledby="schedule-audit-title" aria-busy={busy}>
  <h2 id="schedule-audit-title" class="text-xs font-bold tracking-widest uppercase text-slate-500 mb-2">
    {t('audit.title')}
  </h2>
  <p class="text-[11px] text-slate-400 mb-2">{t('audit.purpose')}</p>

  <div class="grid grid-cols-2 gap-2">
    <label class="text-[11px] text-slate-400">
      {t('audit.profile')}
      <select
        bind:value={profile}
        title={t('audit.profile.hint')}
        class="mt-1 w-full bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-cyan-500 outline-none"
      >
        <option value="default">{t('audit.profile.default')}</option>
        <option value="dcma-style">{t('audit.profile.dcma')}</option>
      </select>
    </label>
    <label class="text-[11px] text-slate-400">
      {t('audit.statusDate')}
      <input
        type="date"
        bind:value={statusDate}
        title={t('audit.statusDate.hint')}
        class="mt-1 w-full bg-slate-950 border border-slate-800 p-1.5 rounded text-xs focus:border-cyan-500 outline-none"
      />
    </label>
  </div>
  <div class="mt-2">
    <Button variant="secondary" size="compact" onclick={run} disabled={busy}>
      {busy ? t('audit.running') : t('audit.run')}
    </Button>
  </div>

  {#if error}
    <p role="alert" class="mt-2 text-xs text-amber-300">{error}</p>
  {/if}

  <!-- Announced politely: the summary when an audit finishes, and the task
       a "Show task" button selected. -->
  <p role="status" class="sr-only">{announcement || summary}</p>

  {#if result}
    <div class="mt-2 p-2 bg-slate-950 rounded text-xs space-y-2">
      <p class="text-slate-200">{summary}</p>
      {#if result.findings.length === 0}
        <p class="text-slate-300">
          {skipped.length > 0 ? t('audit.cleanButSkipped', { count: skipped.length }) : t('audit.clean')}
        </p>
      {/if}

      {#if skipped.length > 0}
        <div>
          <h3 class="text-[10px] uppercase text-slate-500">{t('audit.notEvaluated')}</h3>
          <ul class="mt-1 space-y-1 text-amber-200">
            {#each skipped as rule (rule.rule)}
              <li>{t('audit.notEvaluated.item', { rule: rule.rule, name: ruleName(rule.rule), reason: rule.reason ?? '' })}</li>
            {/each}
          </ul>
        </div>
      {/if}

      {#if result.findings.length > 0}
        <div>
          <h3 class="text-[10px] uppercase text-slate-500">{t('audit.findings')}</h3>
          <ol class="mt-1 space-y-2">
            {#each result.findings as finding, i (i)}
              <li class="border-t border-slate-800 pt-2">
                <p>
                  <span
                    class="font-bold"
                    class:text-red-300={finding.severity === 'error'}
                    class:text-amber-300={finding.severity === 'warning'}
                    class:text-slate-300={finding.severity === 'info'}
                  >{t(severityKey[finding.severity] ?? 'audit.severity.info')}</span>
                  <span class="text-slate-400">· {finding.rule} {ruleName(finding.rule)}</span>
                </p>
                {#if finding.link}
                  <p class="text-slate-300">
                    {t('audit.finding.link', { pred: finding.link.pred, succ: finding.link.succ, type: finding.link.type, lag: formatNumber(finding.link.lag) })}
                  </p>
                {:else if finding.task_id}
                  <p class="text-slate-300">{t('audit.finding.task', { title: finding.task_title || finding.task_id, id: finding.task_id })}</p>
                {/if}
                <p class="text-slate-400">{t('audit.finding.observed', { observed: finding.observed })}</p>
                {#if finding.threshold}
                  <p class="text-slate-400">{t('audit.finding.threshold', { threshold: finding.threshold })}</p>
                {/if}
                {#if finding.explanation}
                  <p class="text-slate-300">{finding.explanation}</p>
                {/if}
                {#if finding.advice}
                  <p class="text-slate-400">{t('audit.finding.advice', { advice: finding.advice })}</p>
                {/if}
                {#if finding.task_id}
                  {#if taskExists(finding.task_id)}
                    <Button
                      variant="secondary"
                      size="compact"
                      class="mt-1"
                      aria-label={t('audit.showTask.label', { title: finding.task_title || finding.task_id })}
                      onclick={() => show(finding.task_id ?? '', finding.task_title ?? '')}
                    >
                      {t('audit.showTask')}
                    </Button>
                  {:else}
                    <p class="text-slate-500 italic">{t('audit.taskMissing')}</p>
                  {/if}
                {/if}
              </li>
            {/each}
          </ol>
        </div>
      {/if}

      {#if result.metrics.length > 0}
        <details>
          <summary class="cursor-pointer text-slate-400">{t('audit.metrics')}</summary>
          <ul class="mt-1 space-y-1 text-slate-300">
            {#each result.metrics as metric (metric.id)}
              <li>
                {t('audit.metric', { id: metric.id, numerator: metric.numerator, denominator: metric.denominator, percent: percent(metric.percent) })}{#if metric.limit}; {metric.limit.at_least
                    ? t('audit.metric.limitAtLeast', { percent: percent(metric.limit.percent) })
                    : t('audit.metric.limitAtMost', { percent: percent(metric.limit.percent) })}{/if}{#if metric.pass != null}; {metric.pass ? t('audit.metric.pass') : t('audit.metric.fail')}{/if}
              </li>
            {/each}
          </ul>
        </details>
      {/if}

      <p class="text-[10px] text-slate-500">{t('audit.revision', { revision: result.source.revision })}</p>
      <p class="text-[10px] text-slate-500">{t('audit.reportHint')}</p>
    </div>
  {/if}
</section>
