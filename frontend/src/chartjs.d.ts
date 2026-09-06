// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import type { AnnotationPluginOptions } from './lib/components/charts/stats_chart_config';

declare module 'chart.js' {
  // The type parameter is required to merge with Chart.js's declaration.
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface PluginOptionsByType<TType extends ChartType> {
    gopmgrAnnotations?: AnnotationPluginOptions;
  }
}
