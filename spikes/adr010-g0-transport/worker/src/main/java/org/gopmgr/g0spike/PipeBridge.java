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
import java.util.UUID;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.TimeUnit;

/**
 * Carries gRPC's HTTP/2 bytes over a Windows named pipe. The JDK can open a
 * named pipe as a file but has no Netty channel for one, so gRPC talks to an
 * in-JVM {@link LocalChannel}, and two threads copy bytes between that
 * channel and the pipe. Nothing listens on TCP or any other OS endpoint.
 *
 * <p>Single mode uses one pipe handle for both directions. Windows serializes
 * synchronous I/O on one handle, so a blocked read may stall writes; the
 * spike measures whether that deadlocks HTTP/2. Dual mode opens two pipe
 * connections, one per direction, so the two never share a handle.
 */
final class PipeBridge implements AutoCloseable {
  private static final byte[] STOP = new byte[0];

  private final EventLoopGroup group = new DefaultEventLoopGroup(2);
  private final LocalAddress address = new LocalAddress("g0-pipe-" + UUID.randomUUID());
  private final String pipeName;
  private final boolean dual;
  private Channel server;
  private RandomAccessFile up;
  private RandomAccessFile down;
  private final BlockingQueue<byte[]> outbound = new LinkedBlockingQueue<>(256);

  private PipeBridge(String pipeName, boolean dual) {
    this.pipeName = pipeName;
    this.dual = dual;
  }

  static PipeBridge start(String pipeName, boolean dual) throws Exception {
    PipeBridge b = new PipeBridge(pipeName, dual);
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

  /** Opens the pipe, retrying while every server instance is busy. */
  private RandomAccessFile openPipe() throws IOException, InterruptedException {
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

  /** Bridges one gRPC connection to the pipe. */
  private final class Pump extends ChannelInboundHandlerAdapter {
    @Override
    public void channelActive(ChannelHandlerContext ctx) throws Exception {
      up = openPipe();
      down = dual ? openPipe() : up;
      Channel ch = ctx.channel();
      Thread reader = new Thread(() -> {
        byte[] buf = new byte[64 * 1024];
        try {
          int n;
          while ((n = down.read(buf)) > 0) {
            ch.writeAndFlush(Unpooled.copiedBuffer(buf, 0, n));
          }
        } catch (IOException ignored) {
          // The pipe closed; the channel close below ends the call.
        }
        ch.close();
      }, "g0-pipe-reader");
      Thread writer = new Thread(() -> {
        try {
          byte[] chunk;
          while ((chunk = outbound.take()) != STOP) {
            up.write(chunk);
          }
        } catch (IOException | InterruptedException ignored) {
          // The pipe or the bridge closed.
        }
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
      outbound.offer(STOP);
      ctx.fireChannelInactive();
    }
  }

  @Override
  public void close() throws Exception {
    outbound.offer(STOP);
    if (server != null) {
      server.close().sync();
    }
    if (up != null) {
      up.close();
    }
    if (down != null && down != up) {
      down.close();
    }
    group.shutdownGracefully(0, 1, TimeUnit.SECONDS);
  }
}
