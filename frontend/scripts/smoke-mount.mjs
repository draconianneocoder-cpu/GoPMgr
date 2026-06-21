// SPDX-FileCopyrightText: 2026 James L. Burns and The PMForge Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// Runtime smoke check for the PMForge frontend.
//
// svelte-check and `vite build` both pass on a whole class of bug that
// still leaves the app dead in the browser: a module that throws at
// load time. The case that motivated this gate was a `$state` rune used
// in a plain `.ts` file (instead of `.svelte.ts`), which throws
// `rune_outside_svelte` the instant the module is imported. App.svelte
// imports it transitively, so #app rendered nothing -- yet check and
// build were green because the throw only happens at runtime.
//
// This script executes the app's real module graph through the actual
// Vite + Svelte compiler (no jsdom, no browser, no new dependency) by
// SSR-loading App.svelte and rendering it. App.svelte's onMount/$effect
// (which touch window.go and dynamic route imports) do not run under
// SSR, so the foundation loads without the Wails backend -- exactly the
// part that must never crash on import. Any load-time or synchronous
// render throw fails the gate.

import { createServer } from 'vite';

const server = await createServer({
  configFile: './vite.config.ts',
  server: { middlewareMode: true },
  appType: 'custom',
  logLevel: 'error',
});

let code = 0;
try {
  // Load svelte/server through the same SSR pipeline as the compiled
  // component so they share one Svelte instance.
  const { render } = await server.ssrLoadModule('svelte/server');

  // Importing App.svelte transitively loads ToastContainer -> toast,
  // session, and every other module in App's synchronous graph. A rune
  // misused in a plain .ts throws here.
  const mod = await server.ssrLoadModule('/src/App.svelte');
  const App = mod.default;
  if (typeof App !== 'function') {
    throw new Error('App.svelte did not export a component as default');
  }

  // Rendering runs the component's synchronous setup. With no Wails
  // bindings present the router falls to its loading branch, which must
  // still produce markup.
  const out = render(App);
  if (!out || typeof out.body !== 'string' || out.body.length === 0) {
    throw new Error('App rendered no HTML body');
  }

  console.log(
    `frontend-smoke: App loaded and rendered (${out.body.length} bytes of HTML).`,
  );
} catch (err) {
  console.error('frontend-smoke: the app failed to load or render.');
  console.error('This means #app would not mount in the browser.');
  console.error(err && err.stack ? err.stack : String(err));
  code = 1;
} finally {
  await server.close();
}

process.exit(code);
