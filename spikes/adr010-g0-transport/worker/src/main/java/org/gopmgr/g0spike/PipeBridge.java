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
import io.netty.util.ReferenceCountUtil;
import java.io.FileNotFoundException;
import java.io.IOException;
import java.io.RandomAccessFile;
import java.nio.ByteBuffer;
import java.nio.channels.AsynchronousFileChannel;
import java.nio.channels.CompletionHandler;
import java.nio.file.FileSystemException;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.util.ArrayDeque;
import java.util.EnumSet;
import java.util.UUID;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.RejectedExecutionException;
import java.util.concurrent.ThreadPoolExecutor;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Carries gRPC's HTTP/2 bytes over one Windows named-pipe connection. The
 * JDK has no Netty channel for named pipes, so gRPC talks to an in-JVM
 * {@link LocalChannel}, and this bridge moves bytes between that channel and
 * the pipe. Nothing listens on TCP or on any other OS endpoint.
 *
 * <p>Rules the bridge keeps:
 *
 * <ul>
 *   <li><b>No event-loop blocking.</b> Pipe reads and writes complete through
 *       callbacks; the pipe is opened before gRPC connects, on the caller's
 *       thread.
 *   <li><b>Bounded buffering.</b> At most one read and one write are in flight.
 *       Above {@link #HIGH_WATER} pending bytes the bridge stops reading from
 *       gRPC (auto-read off) until the pipe drains below {@link #LOW_WATER}.
 *       HTTP/2 flow control bounds what gRPC can queue behind that.
 *   <li><b>One child.</b> The pipe connection serves one gRPC connection; a
 *       second one is refused (one authenticated launch per endpoint).
 *   <li><b>Bounded, idempotent close.</b> {@link #close()} closes the child
 *       and the pipe, which fails any pending I/O, releases queued buffers,
 *       and waits a bounded time for the event loop to terminate.
 * </ul>
 *
 * <p>Modes: {@link Mode#ASYNC_SINGLE} opens one pipe connection through
 * {@link AsynchronousFileChannel} with a private executor, so the channel
 * has its own I/O completion port that the JDK detaches when it closes.
 * {@link Mode#ASYNC_DEFAULT_POOL} uses the JDK's shared default port; its
 * handler thread waits in native code for the life of the JVM, which makes
 * HotSpot wait at exit (G0 finding W2). It exists only as a diagnostic
 * control. {@link Mode#SYNC_SINGLE} and {@link Mode#SYNC_DUAL} use blocking
 * {@link RandomAccessFile} handles, one shared or one per direction.
 */
final class PipeBridge implements AutoCloseable {
  enum Mode {
    SYNC_SINGLE,
    SYNC_DUAL,
    ASYNC_SINGLE,
    ASYNC_DEFAULT_POOL
  }

  static final int READ_BUFFER = 64 * 1024;
  static final int HIGH_WATER = 1 << 20;
  static final int LOW_WATER = 256 << 10;
  static final long OPEN_TIMEOUT_MS = 5_000;
  static final long CLOSE_WAIT_MS = 1_000;

  /** Completion of one read or write. */
  interface Callback {
    /** Bytes moved; -1 for a read at end of stream. */
    void completed(int n);

    void failed(Throwable t);
  }

  /** One full-duplex byte stream whose operations complete through callbacks. */
  interface Duplex extends AutoCloseable {
    /** Reads into {@code dst} once. */
    void read(ByteBuffer dst, Callback cb);

    /** Writes all of {@code src}, then completes. */
    void write(ByteBuffer src, Callback cb);

    /** Idempotent. Pending operations complete with a failure. */
    @Override
    void close();
  }

  private final EventLoopGroup group = new DefaultEventLoopGroup(2);
  private final LocalAddress address = new LocalAddress("g0-pipe-" + UUID.randomUUID());
  private final Duplex pipe;
  private final AtomicBoolean claimed = new AtomicBoolean();
  private final AtomicBoolean closed = new AtomicBoolean();
  private volatile Channel server;
  private volatile Channel child;

  private PipeBridge(Duplex pipe) {
    this.pipe = pipe;
  }

  /** Opens the pipe in {@code mode}, then starts the bridge. */
  static PipeBridge start(String pipeName, Mode mode) throws Exception {
    return start(openDuplex(pipeName, mode));
  }

  /** Starts a bridge over an already-open duplex (also used by the self-test). */
  static PipeBridge start(Duplex pipe) throws InterruptedException {
    PipeBridge b = new PipeBridge(pipe);
    try {
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
    } catch (RuntimeException | InterruptedException e) {
      b.close();
      throw e;
    }
    return b;
  }

  LocalAddress address() {
    return address;
  }

  EventLoopGroup group() {
    return group;
  }

  static Duplex openDuplex(String pipeName, Mode mode) throws IOException, InterruptedException {
    switch (mode) {
      case SYNC_SINGLE: {
        RandomAccessFile f = openSync(pipeName);
        return new BlockingDuplex(f::read, f::write, f, null);
      }
      case SYNC_DUAL: {
        // The host pairs the first connection as worker-to-host.
        RandomAccessFile up = openSync(pipeName);
        RandomAccessFile down;
        try {
          down = openSync(pipeName);
        } catch (IOException | InterruptedException e) {
          up.close();
          throw e;
        }
        return new BlockingDuplex(down::read, up::write, up, down);
      }
      case ASYNC_SINGLE: {
        ExecutorService pool = daemonExecutor("g0-pipe-io");
        try {
          return new AsyncDuplex(openAsync(pipeName, pool), pool);
        } catch (IOException | InterruptedException | RuntimeException e) {
          pool.shutdownNow();
          throw e;
        }
      }
      case ASYNC_DEFAULT_POOL:
        return new AsyncDuplex(openAsync(pipeName, null), null);
      default:
        throw new IllegalStateException("unknown mode " + mode);
    }
  }

  /**
   * Waits between attempts to open a pipe whose instances are all busy. The
   * JDK has no WaitNamedPipe; this is a bounded retry with backoff, not a
   * spin, and it happens once per launch, before gRPC connects.
   */
  private static final class OpenRetry {
    private final long deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(OPEN_TIMEOUT_MS);
    private long delayMs = 5;
    private int attempts;

    <E extends IOException> void again(E busy) throws E, InterruptedException {
      attempts++;
      Timing.count("bridge_open_retries", 1);
      if (System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(delayMs) > deadline) {
        throw busy;
      }
      Thread.sleep(delayMs);
      delayMs = Math.min(delayMs * 2, 200);
    }
  }

  private static RandomAccessFile openSync(String name) throws IOException, InterruptedException {
    OpenRetry retry = new OpenRetry();
    while (true) {
      try {
        return new RandomAccessFile(name, "rw");
      } catch (FileNotFoundException busy) {
        retry.again(busy);
      }
    }
  }

  private static AsynchronousFileChannel openAsync(String name, ExecutorService pool)
      throws IOException, InterruptedException {
    OpenRetry retry = new OpenRetry();
    while (true) {
      try {
        return AsynchronousFileChannel.open(Path.of(name),
            EnumSet.of(StandardOpenOption.READ, StandardOpenOption.WRITE), pool);
      } catch (FileSystemException busy) {
        retry.again(busy);
      }
    }
  }

  static ExecutorService daemonExecutor(String name) {
    return new ThreadPoolExecutor(1, 1, 0, TimeUnit.MILLISECONDS, new LinkedBlockingQueue<>(), r -> {
      Thread t = new Thread(r, name);
      t.setDaemon(true);
      return t;
    });
  }

  /** Runs {@code task} on {@code loop}, unless the bridge has shut it down. */
  private static void onLoop(io.netty.util.concurrent.EventExecutor loop, Runnable task) {
    onLoop(loop, task, null);
  }

  /**
   * Runs {@code task} on {@code loop}; if the loop has already shut down,
   * runs {@code cleanup} here instead, so a late I/O completion still
   * releases what it owns.
   */
  private static void onLoop(io.netty.util.concurrent.EventExecutor loop, Runnable task, Runnable cleanup) {
    try {
      loop.execute(task);
    } catch (RejectedExecutionException closing) {
      Timing.count("bridge_late_completions", 1);
      if (cleanup != null) {
        cleanup.run();
      }
    }
  }

  /** Moves bytes between one gRPC connection and the pipe. Event-loop confined. */
  private final class Pump extends ChannelInboundHandlerAdapter {
    private final ArrayDeque<ByteBuf> pending = new ArrayDeque<>();
    private long pendingBytes; // queued plus in flight
    private ByteBuf inFlight;  // owned by the pipe until its write completes
    private boolean done;
    private ChannelHandlerContext ctx;

    @Override
    public void channelActive(ChannelHandlerContext ctx) {
      if (!claimed.compareAndSet(false, true)) {
        Timing.count("bridge_extra_children_refused", 1);
        ctx.close();
        return;
      }
      this.ctx = ctx;
      child = ctx.channel();
      Timing.mark("bridge_child_active");
      readNext();
      ctx.fireChannelActive();
    }

    /** Issues the one pipe read; the next is issued once gRPC took the bytes. */
    private void readNext() {
      if (done) {
        return;
      }
      ByteBuffer buf = ByteBuffer.allocate(READ_BUFFER);
      pipe.read(buf, new Callback() {
        @Override
        public void completed(int n) {
          onLoop(ctx.executor(), () -> onRead(buf, n));
        }

        @Override
        public void failed(Throwable t) {
          onLoop(ctx.executor(), () -> fail("read", t));
        }
      });
    }

    private void onRead(ByteBuffer buf, int n) {
      if (done) {
        return;
      }
      if (n < 0) {
        Timing.mark("bridge_pipe_eof");
        ctx.close();
        return;
      }
      if (n == 0) {
        readNext();
        return;
      }
      buf.flip();
      ctx.writeAndFlush(Unpooled.wrappedBuffer(buf)).addListener(f -> {
        if (f.isSuccess()) {
          readNext();
        } else {
          fail("deliver", f.cause());
        }
      });
    }

    @Override
    public void channelRead(ChannelHandlerContext ctx, Object msg) {
      if (done || !(msg instanceof ByteBuf b)) {
        ReferenceCountUtil.release(msg);
        return;
      }
      if (!b.isReadable()) {
        // grpc-netty writes an empty buffer when a client transport starts.
        // LocalChannel merges it into the next buffer only when both are
        // queued at read time; read alone, it must not become a zero-length
        // pipe write (G0 finding B9).
        Timing.count("bridge_empty_buffers", 1);
        b.release();
        return;
      }
      pending.add(b);
      pendingBytes += b.readableBytes();
      Timing.peak("bridge_peak_pending_bytes", pendingBytes);
      if (pendingBytes > HIGH_WATER && ctx.channel().config().isAutoRead()) {
        ctx.channel().config().setAutoRead(false);
        Timing.count("bridge_autoread_off", 1);
      }
      if (inFlight == null) {
        writeNext();
      }
    }

    /** Starts the one pipe write, for the oldest queued buffer. */
    private void writeNext() {
      if (done || inFlight != null) {
        return;
      }
      ByteBuf head = pending.poll();
      if (head == null) {
        return;
      }
      inFlight = head;
      int n = head.readableBytes();
      pipe.write(head.nioBuffer(), new Callback() {
        @Override
        public void completed(int written) {
          onLoop(ctx.executor(), () -> onWritten(head, n), head::release);
        }

        @Override
        public void failed(Throwable t) {
          onLoop(ctx.executor(), () -> {
            finishWrite(head);
            fail("write", t);
          }, head::release);
        }
      });
    }

    /** Releases a buffer the pipe has finished with, even after close. */
    private void finishWrite(ByteBuf b) {
      if (inFlight == b) {
        inFlight = null;
      }
      b.release();
    }

    private void onWritten(ByteBuf b, int n) {
      finishWrite(b);
      if (done) {
        return;
      }
      pendingBytes -= n;
      if (!ctx.channel().config().isAutoRead() && pendingBytes <= LOW_WATER) {
        ctx.channel().config().setAutoRead(true);
      }
      writeNext();
    }

    private void fail(String what, Throwable t) {
      if (done) {
        return;
      }
      Timing.count("bridge_" + what + "_failed", 1);
      Timing.note("bridge " + what + " failed: " + t);
      ctx.close();
    }

    /** Drops queued buffers. One still in flight is released when its write ends. */
    private void releasePending() {
      done = true;
      ByteBuf b;
      while ((b = pending.poll()) != null) {
        b.release();
      }
      pendingBytes = 0;
    }

    @Override
    public void channelInactive(ChannelHandlerContext ctx) {
      if (this.ctx != null) {
        Timing.mark("bridge_child_inactive");
        releasePending();
      }
      ctx.fireChannelInactive();
    }

    @Override
    public void exceptionCaught(ChannelHandlerContext ctx, Throwable cause) {
      Timing.count("bridge_exception", 1);
      Timing.note("bridge exception: " + cause);
      ctx.close();
    }
  }

  /**
   * Closes the child channel and the pipe, then shuts down the event loop.
   * Every wait is bounded by {@link #CLOSE_WAIT_MS}.
   */
  @Override
  public void close() {
    if (!closed.compareAndSet(false, true)) {
      return;
    }
    Timing.mark("bridge_close");
    Channel c = child;
    if (c != null) {
      c.close().awaitUninterruptibly(CLOSE_WAIT_MS);
    }
    Channel s = server;
    if (s != null) {
      s.close().awaitUninterruptibly(CLOSE_WAIT_MS);
    }
    Timing.mark("bridge_server_closed");
    pipe.close();
    Timing.mark("bridge_pipe_closed");
    boolean down = group.shutdownGracefully(0, CLOSE_WAIT_MS, TimeUnit.MILLISECONDS)
        .awaitUninterruptibly(2 * CLOSE_WAIT_MS);
    Timing.mark(down ? "bridge_group_terminated" : "bridge_group_not_terminated");
  }

  /** A blocking reader or writer: RandomAccessFile, or a stream in the self-test. */
  interface IoRead {
    int read(byte[] b, int off, int len) throws IOException;
  }

  interface IoWrite {
    void write(byte[] b, int off, int len) throws IOException;
  }

  /**
   * Blocking handles driven by one thread per direction, so a pending read
   * never holds up a write. On Windows, closing a synchronous handle does not
   * cancel a read already blocked on it; that read ends when the host closes
   * its end (recorded as a residual risk of the synchronous modes).
   */
  static final class BlockingDuplex implements Duplex {
    private final IoRead in;
    private final IoWrite out;
    private final AutoCloseable first;
    private final AutoCloseable second;
    private final ExecutorService reader = daemonExecutor("g0-pipe-reader");
    private final ExecutorService writer = daemonExecutor("g0-pipe-writer");
    private final AtomicBoolean closed = new AtomicBoolean();

    BlockingDuplex(IoRead in, IoWrite out, AutoCloseable first, AutoCloseable second) {
      this.in = in;
      this.out = out;
      this.first = first;
      this.second = second;
    }

    @Override
    public void read(ByteBuffer dst, Callback cb) {
      submit(reader, cb, () -> {
        int n = in.read(dst.array(), dst.arrayOffset() + dst.position(), dst.remaining());
        if (n > 0) {
          dst.position(dst.position() + n);
        }
        cb.completed(n);
      });
    }

    @Override
    public void write(ByteBuffer src, Callback cb) {
      byte[] b = new byte[src.remaining()];
      src.get(b);
      submit(writer, cb, () -> {
        out.write(b, 0, b.length);
        cb.completed(b.length);
      });
    }

    private interface Io {
      void run() throws IOException;
    }

    private void submit(ExecutorService exec, Callback cb, Io io) {
      try {
        exec.execute(() -> {
          try {
            io.run();
          } catch (IOException | RuntimeException e) {
            cb.failed(e);
          }
        });
      } catch (RejectedExecutionException e) {
        cb.failed(e);
      }
    }

    @Override
    public void close() {
      if (!closed.compareAndSet(false, true)) {
        return;
      }
      closeQuietly(first);
      closeQuietly(second);
      reader.shutdownNow();
      writer.shutdownNow();
    }
  }

  /** One overlapped pipe connection. Pipes ignore the file position, so it is 0. */
  static final class AsyncDuplex implements Duplex {
    private final AsynchronousFileChannel ch;
    private final ExecutorService pool;
    private final AtomicBoolean closed = new AtomicBoolean();
    /** Reads and write chains issued and not yet finished. Guarded by this. */
    private int outstanding;

    AsyncDuplex(AsynchronousFileChannel ch, ExecutorService pool) {
      this.ch = ch;
      this.pool = pool;
    }

    private synchronized void begin() {
      outstanding++;
    }

    private synchronized void end() {
      if (--outstanding == 0) {
        notifyAll();
      }
    }

    /**
     * Waits until every issued operation has delivered its completion, for at
     * most {@code timeoutMs}. Woken by the last completion; no polling.
     */
    private synchronized boolean awaitIdle(long timeoutMs) throws InterruptedException {
      long deadline = System.nanoTime() + TimeUnit.MILLISECONDS.toNanos(timeoutMs);
      while (outstanding > 0) {
        long left = deadline - System.nanoTime();
        if (left <= 0) {
          return false;
        }
        TimeUnit.NANOSECONDS.timedWait(this, left);
      }
      return true;
    }

    /**
     * Refuses an operation issued after {@link #close()}. Its completion would
     * go to an executor that may already be shut down, and the JDK would then
     * fail on its own thread instead of calling the handler.
     */
    private boolean refuseIfClosed(Callback cb) {
      if (!closed.get()) {
        return false;
      }
      Timing.count("bridge_ops_after_close", 1);
      cb.failed(new java.nio.channels.ClosedChannelException());
      return true;
    }

    @Override
    public void read(ByteBuffer dst, Callback cb) {
      if (refuseIfClosed(cb)) {
        return;
      }
      begin();
      try {
        ch.read(dst, 0, null, new CompletionHandler<Integer, Void>() {
          @Override
          public void completed(Integer n, Void a) {
            end();
            cb.completed(n);
          }

          @Override
          public void failed(Throwable t, Void a) {
            end();
            cb.failed(t);
          }
        });
      } catch (RuntimeException e) {
        end();
        cb.failed(e);
      }
    }

    /**
     * One chain of writes until {@code src} is empty; counted as one
     * operation. An empty {@code src} completes at once with no I/O: the JDK
     * would complete it with 0 bytes, which reads as a write that made no
     * progress.
     */
    @Override
    public void write(ByteBuffer src, Callback cb) {
      if (!src.hasRemaining()) {
        cb.completed(0);
        return;
      }
      if (refuseIfClosed(cb)) {
        return;
      }
      begin();
      CompletionHandler<Integer, Void> h = new CompletionHandler<>() {
        private int total;

        @Override
        public void completed(Integer n, Void a) {
          if (n <= 0) {
            end();
            cb.failed(new IOException("pipe write made no progress (" + n + " of "
                + (total + src.remaining()) + " bytes written in this call chain)"));
            return;
          }
          total += n;
          if (!src.hasRemaining()) {
            end();
            cb.completed(total);
            return;
          }
          try {
            ch.write(src, 0, null, this);
          } catch (RuntimeException e) {
            end();
            cb.failed(e);
          }
        }

        @Override
        public void failed(Throwable t, Void a) {
          end();
          cb.failed(t);
        }
      };
      try {
        ch.write(src, 0, null, h);
      } catch (RuntimeException e) {
        end();
        cb.failed(e);
      }
    }

    /**
     * Closing the channel fails pending operations and, for a private
     * executor, detaches the channel's completion port so its handler thread
     * leaves native code.
     *
     * <p>The JDK hands the failure of each aborted operation to the executor
     * from its completion-port thread, after {@code close} may already have
     * returned. Shutting the executor down first makes that hand-off fail
     * with {@code ShutdownChannelGroupException} on the JDK's thread, and the
     * operation's handler (and any buffer it releases) never runs. So this
     * waits, bounded, until every issued operation has completed, then shuts
     * the executor down with a bounded wait.
     */
    @Override
    public void close() {
      if (!closed.compareAndSet(false, true)) {
        return;
      }
      closeQuietly(ch);
      if (pool == null) {
        return;
      }
      try {
        if (!awaitIdle(CLOSE_WAIT_MS)) {
          Timing.count("bridge_pending_io_abandoned", 1);
        }
        pool.shutdown();
        if (!pool.awaitTermination(CLOSE_WAIT_MS, TimeUnit.MILLISECONDS)) {
          Timing.count("bridge_pool_forced", 1);
          pool.shutdownNow();
        }
      } catch (InterruptedException e) {
        pool.shutdownNow();
        Thread.currentThread().interrupt();
      }
    }
  }

  static void closeQuietly(AutoCloseable c) {
    if (c == null) {
      return;
    }
    try {
      c.close();
    } catch (Exception e) {
      Timing.count("bridge_close_error", 1);
    }
  }
}
