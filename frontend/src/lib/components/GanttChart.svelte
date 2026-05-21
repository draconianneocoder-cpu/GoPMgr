<!--
SPDX-FileCopyrightText: 2026 The PMForge Contributors
SPDX-License-Identifier: GPL-3.0-or-later
-->
<script lang="ts">
  // GanttChart renders kernel-computed tasks (ES/EF/IsCritical) as
  // accessible progress bars. Each bar is keyboard-focusable and
  // announces its full schedule via a visually-hidden span.
  //
  // Source-of-truth for the a11y attributes: V1 Release Gate #2.
  let {
    tasks = [] as KernelTask[],
    pixelsPerDay = 40,
  } = $props();

  const totalDuration = $derived(
    tasks.length > 0 ? Math.max(...tasks.map((t) => t.ef)) : 0
  );
</script>

<div
  class="gantt-container overflow-x-auto bg-slate-950 p-4 border border-slate-800 rounded-lg"
  role="application"
  aria-label="Project Schedule Gantt Chart"
>
  {#if tasks.length === 0}
    <p class="text-slate-500 text-sm">No tasks scheduled.</p>
  {:else}
    <div
      class="task-rows space-y-2 relative"
      style="width: {totalDuration * pixelsPerDay}px; min-height: {tasks.length * 32}px"
    >
      {#each tasks as task, i (task.id)}
        <div
          class="task-bar absolute h-6 rounded flex items-center px-2 text-[10px] font-bold focus:outline focus:outline-2 focus:outline-cyan-300"
          style="top: {i * 32}px; left: {task.es * pixelsPerDay}px; width: {(task.ef - task.es) * pixelsPerDay}px; background-color: {task.is_critical ? '#ef4444' : '#00D4FF'}; color: #0b1220;"
          role="progressbar"
          aria-valuenow={task.ef - task.es}
          aria-valuemin="0"
          aria-valuemax={totalDuration}
          aria-label={`Task ${task.title}`}
          tabindex="0"
        >
          <span class="sr-only">
            {task.title} starts on day {task.es} and ends on day {task.ef}.
            {task.is_critical ? 'On the critical path.' : `Float: ${task.float} days.`}
          </span>
          {task.title}
        </div>
      {/each}
    </div>
  {/if}
</div>
