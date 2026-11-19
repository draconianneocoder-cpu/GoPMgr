// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// saveChartWhenDocumentChanges writes a chart only when `data` differs from
// the document string the editor already accepted. Opening a chart accepts
// the loaded document, so a layout refresh does not write it.
export async function saveChartWhenDocumentChanges<T extends { data: string }>(
  chart: T,
  data: string,
  clean: string,
  save: (next: T) => Promise<T>,
  force = false,
): Promise<{ chart: T; clean: string; wrote: boolean }> {
  if (!force && data === clean) {
    return { chart, clean, wrote: false };
  }
  const updated = await save({ ...chart, data });
  return { chart: updated, clean: data, wrote: true };
}
