// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package org.gopmgr.g0spike;

import io.netty.bootstrap.ServerBootstrap;
import io.netty.buffer.ByteBuf;
import io.netty.buffer.Unpooled;
import io.netty.channel.Channel;
import io.netty.channel.ChannelHandlerContext;
import io.netty.channel.ChannelInboundHandlerAdapter;
import io.netty.channel.ChannelInitializer;
import io.netty.channel.DefaultEventLoopGroup;
import io.netty.channel.EventLoopGroup;
import io.netty.channel.local.LocalAddress;
import io.netty.channel.local.LocalChannel;
import io.netty.channel.local.LocalServerChannel;
import java.io.FileNotFoundException;
import java.io.IOException;
import java.io.RandomAccessFile;
import java.nio.ByteBuffer;
import java.nio.channels.AsynchronousFileChannel;
import java.nio.file.FileSystemException;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.util.UUID;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.TimeUnit;

/**
 * Carries gRPC's HTTP/2 bytes over a Windows named pipe. The JDK can open a
 * named pipe as a file but has no Netty channel for one, so gRPC talks to an
 * in-JVM {@link LocalChannel}, and two threads copy bytes between that
 * channel and the pipe. Nothing listens on TCP or any other OS endpoint.
 *
 * <p>{@link Mode#SYNC_SINGLE} uses one synchronous pipe handle for both
 * directions. Windows serializes synchronous I/O on one handle, so a blocked
 * read may stall writes; the spike measures whether that deadlocks HTTP/2.
 * {@link Mode#SYNC_DUAL} opens two pipe connections, one per direction, so
 * the two never share a handle. {@link Mode#ASYNC_SINGLE} opens one pipe
 * connection through {@link AsynchronousFileChannel}, which the JDK opens for
 * overlapped I/O on Windows, so a pending read does not block a write.
 */
final class PipeBridge implements AutoCloseable {
  private static final byte[] STOP = new byte[0];

  /** How the bridge opens and uses the pipe. */
  enum Mode {
    SYNC_SINGLE,
    SYNC_DUAL,
    ASYNC_SINGLE
  }

  /** One pipe connection as a byte stream in each direction. */
  private interface Duplex extends AutoCloseable {
    /** Reads up to buf.length bytes; returns -1 at end of stream. */
    int read(byte[] buf) throws IOException;

    void write(byte[] b) throws IOException;

    @Override
    void close() throws IOException;
  }

  private final EventLoopGroup group = new DefaultEventLoopGroup(2);
  private final LocalAddress address = new LocalAddress("g0-pipe-" + UUID.randomUUID());
  private final String pipeName;
  private final Mode mode;
  private Channel server;
  private volatile Duplex pipe;
  private final BlockingQueue<byte[]> outbound = new LinkedBlockingQueue<>(256);

  private PipeBridge(String pipeName, Mode mode) {
    this.pipeName = pipeName;
    this.mode = mode;
  }

  static PipeBridge start(String pipeName, Mode mode) throws Exception {
    PipeBridge b = new PipeBridge(pipeName, mode);
    b.server = new ServerBootstrap()
        .group(b.group)
        .channel(LocalServerChannel.class)
        .childHandler(new ChannelInitializer<LocalChannel>() {
          @Override
          protected void initChannel(LocalChannel ch) {
            ch.pipeline().addLast(b.new Pump());
          }
        })
        .bind(b.address).sync().channel();
    return b;
  }

  LocalAddress address() {
    return address;
  }

  EventLoopGroup group() {
    return group;
  }

  /** Opens a synchronous pipe handle, retrying while every instance is busy. */
  private RandomAccessFile openSync() throws IOException, InterruptedException {
    long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5);
    while (true) {
      try {
        return new RandomAccessFile(pipeName, "rw");
      } catch (FileNotFoundException busy) {
        if (System.nanoTime() > deadline) {
          throw busy;
        }
        Thread.sleep(20);
      }
    }
  }

  /** Opens an overlapped pipe handle, retrying while every instance is busy. */
  private AsynchronousFileChannel openAsync() throws IOException, InterruptedException {
    long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(5);
    while (true) {
      try {
        return AsynchronousFileChannel.open(
            Path.of(pipeName), StandardOpenOption.READ, StandardOpenOption.WRITE);
      } catch (FileSystemException busy) {
        if (System.nanoTime() > deadline) {
          throw busy;
        }
        Thread.sleep(20);
      }
    }
  }

  private Duplex openDuplex() throws IOException, InterruptedException {
    switch (mode) {
      case SYNC_SINGLE: {
        RandomAccessFile f = openSync();
        return new SyncDuplex(f, f);
      }
      case SYNC_DUAL: {
        // The host pairs the first connection as worker-to-host.
        RandomAccessFile up = openSync();
        RandomAccessFile down;
        try {
          down = openSync();
        } catch (IOException | InterruptedException e) {
          up.close();
          throw e;
        }
        return new SyncDuplex(up, down);
      }
      case ASYNC_SINGLE:
        return new AsyncDuplex(openAsync());
      default:
        throw new IllegalStateException("unknown mode " + mode);
    }
  }

  /** Synchronous handles: one shared, or one per direction. */
  private static final class SyncDuplex implements Duplex {
    private final RandomAccessFile up;
    private final RandomAccessFile down;

    SyncDuplex(RandomAccessFile up, RandomAccessFile down) {
      this.up = up;
      this.down = down;
    }

    @Override
    public int read(byte[] buf) throws IOException {
      return down.read(buf);
    }

    @Override
    public void write(byte[] b) throws IOException {
      up.write(b);
    }

    @Override
    public void close() throws IOException {
      try {
        up.close();
      } finally {
        if (down != up) {
          down.close();
        }
      }
    }
  }

  /**
   * One overlapped handle. Pipes ignore the file position, so every call
   * passes 0. A read and a write may be outstanding at the same time.
   */
  private static final class AsyncDuplex implements Duplex {
    private final AsynchronousFileChannel ch;

    AsyncDuplex(AsynchronousFileChannel ch) {
      this.ch = ch;
    }

    @Override
    public int read(byte[] buf) throws IOException {
      return await(ch.read(ByteBuffer.wrap(buf), 0));
    }

    @Override
    public void write(byte[] b) throws IOException {
      ByteBuffer bb = ByteBuffer.wrap(b);
      while (bb.hasRemaining()) {
        if (await(ch.write(bb, 0)) <= 0) {
          throw new IOException("pipe write made no progress");
        }
      }
    }

    private static int await(java.util.concurrent.Future<Integer> f) throws IOException {
      try {
        return f.get();
      } catch (InterruptedException e) {
        Thread.currentThread().interrupt();
        throw new IOException("interrupted", e);
      } catch (ExecutionException e) {
        throw e.getCause() instanceof IOException io ? io : new IOException(e.getCause());
      }
    }

    @Override
    public void close() throws IOException {
      ch.close();
    }
  }

  /** Bridges one gRPC connection to the pipe. */
  private final class Pump extends ChannelInboundHandlerAdapter {
    @Override
    public void channelActive(ChannelHandlerContext ctx) throws Exception {
      Timing.mark("bridge_child_active");
      Duplex p = openDuplex();
      pipe = p;
      Timing.mark("bridge_pipe_opened");
      Channel ch = ctx.channel();
      Thread reader = new Thread(() -> {
        byte[] buf = new byte[64 * 1024];
        try {
          int n;
          while ((n = p.read(buf)) > 0) {
            ch.writeAndFlush(Unpooled.copiedBuffer(buf, 0, n));
          }
        } catch (IOException ignored) {
          // The pipe closed; the channel close below ends the call.
        }
        Timing.mark("bridge_reader_exit");
        ch.close();
      }, "g0-pipe-reader");
      Thread writer = new Thread(() -> {
        try {
          byte[] chunk;
          while ((chunk = outbound.take()) != STOP) {
            p.write(chunk);
          }
        } catch (IOException | InterruptedException ignored) {
          // The pipe or the bridge closed.
        }
        Timing.mark("bridge_writer_exit");
        ch.close();
      }, "g0-pipe-writer");
      reader.setDaemon(true);
      writer.setDaemon(true);
      reader.start();
      writer.start();
      ctx.fireChannelActive();
    }

    @Override
    public void channelRead(ChannelHandlerContext ctx, Object msg) throws Exception {
      ByteBuf b = (ByteBuf) msg;
      try {
        byte[] copy = new byte[b.readableBytes()];
        b.readBytes(copy);
        outbound.put(copy);
      } finally {
        b.release();
      }
    }

    @Override
    public void channelInactive(ChannelHandlerContext ctx) {
      Timing.mark(outbound.offer(STOP) ? "bridge_child_inactive" : "bridge_child_inactive_stop_dropped");
      ctx.fireChannelInactive();
    }
  }

  @Override
  public void close() throws Exception {
    Timing.mark(outbound.offer(STOP) ? "bridge_close" : "bridge_close_stop_dropped");
    if (server != null) {
      server.close().sync();
    }
    Timing.mark("bridge_server_closed");
    Duplex p = pipe;
    if (p != null) {
      p.close();
    }
    Timing.mark("bridge_pipe_closed");
    group.shutdownGracefully(0, 1, TimeUnit.SECONDS);
  }
}
