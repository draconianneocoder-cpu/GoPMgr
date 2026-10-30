// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package org.gopmgr.g0spike;

/**
 * A grandchild process for the G0 process-tree scenarios: it only sleeps.
 * The worker starts it to check that the host can end the whole tree.
 */
public final class Sleeper {
  private Sleeper() {}

  public static void main(String[] args) throws InterruptedException {
    long seconds = args.length > 0 ? Long.parseLong(args[0]) : 120;
    Thread.sleep(seconds * 1_000);
  }
}
