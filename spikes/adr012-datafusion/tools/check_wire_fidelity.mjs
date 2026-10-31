// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

// DF0's existing typed-cell fixture format, not a production Wails DTO.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

try {
  const { actual, expected } = JSON.parse(readFileSync(0, 'utf8'));
  assert.deepStrictEqual(actual, expected);
  assert(Array.isArray(actual.columns));
  assert(Array.isArray(actual.rows));
  for (const row of actual.rows) {
    assert.equal(row.length, actual.columns.length);
    for (const cell of row) {
      assert.equal(typeof cell.k, 'string');
      assert.equal(typeof cell.v, 'string');
      if (cell.k === 'int') {
        assert.equal(BigInt(cell.v).toString(), cell.v);
      }
      // Nested JSON stays opaque text. JSON.parse(cell.v) would round a
      // nested int64; neither that conversion nor native UI use is qualified.
    }
  }
  process.stdout.write(JSON.stringify({ rows: actual.rows.length, columns: actual.columns.length }));
} catch {
  process.stderr.write('WIRE_MISMATCH\n');
  process.exitCode = 1;
}
