// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package org.gopmgr.g0spike;

import java.util.Queue;
import java.util.concurrent.ConcurrentLinkedQueue;

/**
 * Diagnostic timing marks for the G0 cancel investigation. Each mark records
 * microseconds since the worker started, on the JVM's monotonic clock. The
 * marks are printed once, to stderr, by a shutdown hook, as one line:
 * {@code g0-timing: label=micros,label=micros,...}. Labels are fixed strings;
 * no data from the host or the session is ever printed.
 */
final class Timing {
  private static final long START = System.nanoTime();
  private static final Queue<String> MARKS = new ConcurrentLinkedQueue<>();

  static {
    Runtime.getRuntime().addShutdownHook(new Thread(() -> {
      mark("shutdown_hook");
      System.err.println("g0-timing: " + String.join(",", MARKS));
      System.err.flush();
    }, "g0-timing"));
  }

  private Timing() {}

  /** Records that {@code label} happened now. */
  static void mark(String label) {
    MARKS.add(label + "=" + (System.nanoTime() - START) / 1_000);
  }
}
