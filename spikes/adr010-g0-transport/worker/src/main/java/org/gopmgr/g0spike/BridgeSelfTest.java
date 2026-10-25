// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package org.gopmgr.g0spike;

import io.netty.bootstrap.Bootstrap;
import io.netty.buffer.ByteBuf;
import io.netty.buffer.Unpooled;
import io.netty.channel.Channel;
import io.netty.channel.ChannelHandlerContext;
import io.netty.channel.ChannelInboundHandlerAdapter;
import io.netty.channel.ChannelInitializer;
import io.netty.channel.local.LocalChannel;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.channels.Channels;
import java.nio.channels.Pipe;
import java.security.MessageDigest;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Exercises {@link PipeBridge} on any OS, with an in-memory duplex in place
 * of the Windows pipe. It drives the real bridge code: event-loop handling,
 * buffering limits, close and child refusal. Prints one line per test:
 * {@code g0-selftest: name=PASS|FAIL detail}; exits 1 if any test fails.
 */
public final class BridgeSelfTest {
  private BridgeSelfTest() {}

  /** The far end of the in-memory "pipe": what the Go host would be. */
  private static final class Remote {
    final Pipe toBridge;
    final Pipe toRemote;
    final InputStream in;
    final OutputStream out;

    Remote() throws IOException {
      toBridge = Pipe.open();
      toRemote = Pipe.open();
      in = Channels.newInputStream(toRemote.source());
      out = Channels.newOutputStream(toBridge.sink());
    }

    PipeBridge.Duplex duplex() {
      InputStream bridgeIn = Channels.newInputStream(toBridge.source());
      OutputStream bridgeOut = Channels.newOutputStream(toRemote.sink());
      return new PipeBridge.BlockingDuplex(bridgeIn::read, bridgeOut::write, bridgeIn, bridgeOut);
    }

    void close() {
      PipeBridge.closeQuietly(in);
      PipeBridge.closeQuietly(out);
    }
  }

  /** A gRPC stand-in on the LocalChannel side: digests what it receives. */
  private static final class Client extends ChannelInboundHandlerAdapter {
    final MessageDigest digest = Worker.sha256();
    final AtomicLong received = new AtomicLong();
    final CountDownLatch inactive = new CountDownLatch(1);
    volatile long expect = Long.MAX_VALUE;
    final CountDownLatch gotAll = new CountDownLatch(1);

    @Override
    public void channelRead(ChannelHandlerContext ctx, Object msg) {
      ByteBuf b = (ByteBuf) msg;
      try {
        byte[] a = new byte[b.readableBytes()];
        b.readBytes(a);
        synchronized (digest) {
          digest.update(a);
        }
        if (received.addAndGet(a.length) >= expect) {
          gotAll.countDown();
        }
      } finally {
        b.release();
      }
    }

    @Override
    public void channelInactive(ChannelHandlerContext ctx) {
      inactive.countDown();
    }
  }

  private static Channel connect(PipeBridge bridge, Client handler) throws InterruptedException {
    return new Bootstrap()
        .group(bridge.group())
        .channel(LocalChannel.class)
        .handler(new ChannelInitializer<LocalChannel>() {
          @Override
          protected void initChannel(LocalChannel ch) {
            ch.pipeline().addLast(handler);
          }
        })
        .connect(bridge.address()).sync().channel();
  }

  private static byte[] block(int i, int size) {
    byte[] b = new byte[size];
    for (int j = 0; j < size; j++) {
      b[j] = (byte) (i * 31 + j);
    }
    return b;
  }

  private static final List<String> FAILURES = new ArrayList<>();

  private static void report(String name, boolean ok, String detail) {
    System.out.println("g0-selftest: " + name + "=" + (ok ? "PASS" : "FAIL") + " " + detail);
    if (!ok) {
      FAILURES.add(name);
    }
  }

  /** 16 MiB each way at once; both digests must match. */
  static void integrity() throws Exception {
    final int blocks = 256;
    final int size = 64 * 1024;
    Remote remote = new Remote();
    PipeBridge bridge = PipeBridge.start(remote.duplex());
    Client client = new Client();
    client.expect = (long) blocks * size;
    Channel ch = connect(bridge, client);
    MessageDigest sentToRemote = Worker.sha256();
    MessageDigest sentToClient = Worker.sha256();
    MessageDigest gotAtRemote = Worker.sha256();
    Thread reader = new Thread(() -> {
      byte[] buf = new byte[size];
      long left = (long) blocks * size;
      try {
        while (left > 0) {
          int n = remote.in.read(buf, 0, (int) Math.min(buf.length, left));
          if (n < 0) {
            break;
          }
          gotAtRemote.update(buf, 0, n);
          left -= n;
        }
      } catch (IOException ignored) {
        // Reported by the digest mismatch.
      }
    });
    Thread writer = new Thread(() -> {
      try {
        for (int i = 0; i < blocks; i++) {
          byte[] b = block(i + 7, size);
          sentToClient.update(b);
          remote.out.write(b);
        }
        remote.out.flush();
      } catch (IOException ignored) {
        // Reported by the digest mismatch.
      }
    });
    reader.start();
    writer.start();
    for (int i = 0; i < blocks; i++) {
      byte[] b = block(i, size);
      sentToRemote.update(b);
      ch.writeAndFlush(Unpooled.wrappedBuffer(b)).sync();
    }
    boolean all = client.gotAll.await(30, TimeUnit.SECONDS);
    reader.join(30_000);
    writer.join(30_000);
    boolean up = Arrays.equals(sentToRemote.digest(), gotAtRemote.digest());
    boolean down;
    synchronized (client.digest) {
      down = all && Arrays.equals(sentToClient.digest(), client.digest.digest());
    }
    ch.close().sync();
    bridge.close();
    remote.close();
    report("integrity", up && down, "to_pipe=" + (up ? "ok" : "mismatch") + " from_pipe=" + (down ? "ok" : "mismatch"));
  }

  /**
   * The far end stalls; the bridge must stop reading from gRPC instead of
   * buffering without limit, then deliver everything once the far end reads.
   */
  static void backpressure() throws Exception {
    final int blocks = 128;
    final int size = 64 * 1024;
    Remote remote = new Remote();
    PipeBridge bridge = PipeBridge.start(remote.duplex());
    Client client = new Client();
    Channel ch = connect(bridge, client);
    Timing.reset("bridge_peak_pending_bytes");
    long before = statOf("bridge_autoread_off");
    for (int i = 0; i < blocks; i++) {
      ch.writeAndFlush(Unpooled.wrappedBuffer(block(i, size)));
    }
    Thread.sleep(500); // far end not reading
    long peak = statOf("bridge_peak_pending_bytes");
    long offs = statOf("bridge_autoread_off") - before;
    long left = (long) blocks * size;
    byte[] buf = new byte[size];
    long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(30);
    while (left > 0 && System.nanoTime() < deadline) {
      int n = remote.in.read(buf, 0, (int) Math.min(buf.length, left));
      if (n < 0) {
        break;
      }
      left -= n;
    }
    ch.close().sync();
    bridge.close();
    remote.close();
    long bound = PipeBridge.HIGH_WATER + size;
    boolean ok = peak <= bound && offs >= 1 && left == 0;
    report("backpressure", ok, "peak_pending_bytes=" + peak + " bound=" + bound
        + " autoread_off=" + offs + " undelivered=" + left);
  }

  /**
   * Close while the far end is stalled and buffers are queued and in flight:
   * close must be bounded, the event loop must terminate, and every buffer the
   * bridge held must be released.
   */
  static void closeWithPendingIo() throws Exception {
    final int blocks = 64;
    final int size = 64 * 1024;
    Remote remote = new Remote();
    PipeBridge bridge = PipeBridge.start(remote.duplex());
    Client client = new Client();
    Channel ch = connect(bridge, client);
    List<ByteBuf> sent = new ArrayList<>();
    for (int i = 0; i < blocks; i++) {
      ByteBuf b = Unpooled.wrappedBuffer(block(i, size));
      b.retain(); // one reference for the bridge, one kept here to check release
      sent.add(b);
      ch.writeAndFlush(b);
    }
    Thread.sleep(300); // queued behind a stalled far end
    long t0 = System.nanoTime();
    bridge.close();
    long closeMs = (System.nanoTime() - t0) / 1_000_000;
    boolean terminated = bridge.group().isTerminated();
    remote.close();
    // Let any late completion run its cleanup.
    Thread.sleep(200);
    int leaked = 0;
    for (ByteBuf b : sent) {
      if (b.refCnt() != 1) {
        leaked++;
      }
      b.release();
    }
    boolean ok = closeMs < 3 * PipeBridge.CLOSE_WAIT_MS && terminated && leaked == 0;
    report("close-with-pending-io", ok, "close_ms=" + closeMs + " group_terminated=" + terminated
        + " buffers_not_released=" + leaked);
  }

  /** The pipe serves one gRPC connection; a second one is refused at once. */
  static void secondChildRefused() throws Exception {
    Remote remote = new Remote();
    PipeBridge bridge = PipeBridge.start(remote.duplex());
    long before = statOf("bridge_extra_children_refused");
    Client first = new Client();
    Client second = new Client();
    Channel a = connect(bridge, first);
    connect(bridge, second);
    boolean closed = second.inactive.await(5, TimeUnit.SECONDS);
    boolean firstAlive = a.isActive();
    long refused = statOf("bridge_extra_children_refused") - before;
    a.close().sync();
    bridge.close();
    remote.close();
    report("second-child-refused", closed && firstAlive && refused == 1,
        "second_closed=" + closed + " first_alive=" + firstAlive + " refused=" + refused);
  }

  private static long statOf(String key) {
    return Timing.stat(key);
  }

  public static void main(String[] args) throws Exception {
    integrity();
    backpressure();
    closeWithPendingIo();
    secondChildRefused();
    System.out.println("g0-selftest: summary=" + (FAILURES.isEmpty() ? "PASS" : "FAIL " + FAILURES));
    System.out.flush();
    System.exit(FAILURES.isEmpty() ? 0 : 1);
  }
}
