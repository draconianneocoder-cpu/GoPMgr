// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

import { Events } from '@wailsio/runtime';

// Keep the runtime envelope at the transport boundary. Callers receive only
// data, and own the runtime's unsubscribe function for their mount lifetime.
export const desktopRuntime = {
  EventsOn(name: string, callback: (data: unknown) => void): () => void {
    return Events.On(name, (event) => callback(event.data));
  },
};
