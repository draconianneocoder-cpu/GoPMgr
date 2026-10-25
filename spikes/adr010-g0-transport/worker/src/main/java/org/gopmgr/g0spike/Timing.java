// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package org.gopmgr.g0spike;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Queue;
import java.util.Set;
import java.util.TreeMap;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentLinkedQueue;

/**
 * Diagnostics for the G0 investigation, printed once to stderr by a
 * shutdown hook. Three lines, all made of fixed labels and numbers; no data
 * from the host or the session is ever printed:
 *
 * <ul>
 *   <li>{@code g0-timing: label=micros,...}: microseconds since the worker
 *       started, on the JVM's monotonic clock;
 *   <li>{@code g0-stats: key=value,...}: counters such as peak buffered bytes;
 *   <li>{@code g0-native-at-exit: thread@frame;...}: threads whose top frame
 *       was a native I/O wait when the worker called System.exit. HotSpot
 *       waits at exit for threads in native code (VM_Exit), so these explain
 *       slow exits.
 * </ul>
 */
final class Timing {
  private static final long START = System.nanoTime();
  private static final Queue<String> MARKS = new ConcurrentLinkedQueue<>();
  private static final Map<String, Long> STATS = new ConcurrentHashMap<>();
  private static volatile String nativeAtExit = "";

  /** Native methods in which a thread waits inside the VM, not in native code. */
  private static final Set<String> VM_WAITS = Set.of(
      "java.lang.Object.wait0",
      "java.lang.ref.Reference.waitForReferencePendingList",
      "jdk.internal.misc.Unsafe.park",
      "java.lang.Thread.sleep0",
      "java.lang.Thread.sleepNanos0",
      "java.lang.Thread.yield0");

  static {
    Runtime.getRuntime().addShutdownHook(new Thread(() -> {
      mark("shutdown_hook");
      StringBuilder stats = new StringBuilder();
      new TreeMap<>(STATS).forEach((k, v) -> stats.append(stats.length() == 0 ? "" : ",").append(k).append('=').append(v));
      System.err.println("g0-timing: " + String.join(",", MARKS));
      System.err.println("g0-stats: " + stats);
      System.err.println("g0-native-at-exit: " + nativeAtExit);
      System.err.flush();
    }, "g0-timing"));
  }

  private Timing() {}

  /** Records that {@code label} happened now. */
  static void mark(String label) {
    MARKS.add(label + "=" + (System.nanoTime() - START) / 1_000);
  }

  /** Adds {@code delta} to counter {@code key}. */
  static void count(String key, long delta) {
    STATS.merge(key, delta, Long::sum);
  }

  /** Keeps the largest {@code value} seen for {@code key}. */
  static void peak(String key, long value) {
    STATS.merge(key, value, Math::max);
  }

  /** Current value of counter {@code key}, or 0. */
  static long stat(String key) {
    return STATS.getOrDefault(key, 0L);
  }

  /** Clears counter {@code key} (self-test only). */
  static void reset(String key) {
    STATS.remove(key);
  }

  /** Records which threads sit in a native wait right now. */
  static void snapshotNativeThreads() {
    List<String> found = new ArrayList<>();
    Thread self = Thread.currentThread();
    for (Map.Entry<Thread, StackTraceElement[]> e : Thread.getAllStackTraces().entrySet()) {
      Thread t = e.getKey();
      StackTraceElement[] st = e.getValue();
      if (t == self || !t.isAlive() || st.length == 0 || !st[0].isNativeMethod()) {
        continue;
      }
      String frame = st[0].getClassName() + "." + st[0].getMethodName();
      if (!VM_WAITS.contains(frame)) {
        found.add(t.getName().replace(';', '_').replace(',', '_') + "@" + frame);
      }
    }
    found.sort(null);
    nativeAtExit = String.join(";", found);
  }
}
