// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package org.gopmgr.g0spike;

import com.google.protobuf.ByteString;
import io.grpc.ManagedChannel;
import io.grpc.Metadata;
import io.grpc.Status;
import io.grpc.netty.NettyChannelBuilder;
import io.grpc.stub.ClientCallStreamObserver;
import io.grpc.stub.ClientResponseObserver;
import io.grpc.stub.MetadataUtils;
import io.grpc.stub.StreamObserver;
import io.netty.channel.EventLoopGroup;
import io.netty.channel.unix.DomainSocketAddress;
import java.io.DataInputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.PrintStream;
import java.net.UnixDomainSocketAddress;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import org.gopmgr.g0spike.pb.Bootstrap;
import org.gopmgr.g0spike.pb.Completed;
import org.gopmgr.g0spike.pb.HostCommand;
import org.gopmgr.g0spike.pb.Pong;
import org.gopmgr.g0spike.pb.Record;
import org.gopmgr.g0spike.pb.WorkerEvent;
import org.gopmgr.g0spike.pb.WorkerHello;
import org.gopmgr.g0spike.pb.WorkerSessionGrpc;

/**
 * ADR-010 G0 spike worker. Non-production. It reads a one-shot bootstrap
 * from stdin (INT-02), connects to the Go host as a gRPC client over the
 * requested local IPC transport, and runs the scenario the host sends.
 */
public final class Worker {
  static final int EXIT_OK = 0;
  static final int EXIT_BOOTSTRAP = 2;
  static final int EXIT_CANCELLED = 3;
  static final int EXIT_REJECTED = 4;
  static final int EXIT_ERROR = 5;
  static final int EXIT_EXPIRED = 6;

  /** Largest bootstrap the worker accepts, in bytes. */
  static final int MAX_BOOTSTRAP = 4096;

  /** Domain-separation label for the connection proof. */
  static final String PROOF_LABEL = "gopmgr-g0spike/connect/v1";

  static final Metadata.Key<String> PROOF_KEY =
      Metadata.Key.of("x-g0-proof", Metadata.ASCII_STRING_MARSHALLER);

  private Worker() {}

  public static void main(String[] args) {
    Timing.mark("main");
    Bootstrap boot;
    try {
      boot = readBootstrap(System.in);
    } catch (IOException | RuntimeException e) {
      System.err.println("bootstrap rejected: " + e.getClass().getSimpleName() + ": " + e.getMessage());
      System.exit(EXIT_BOOTSTRAP);
      return;
    }
    if (boot.getVersion() != 1) {
      System.err.println("bootstrap rejected: unsupported version " + boot.getVersion());
      System.exit(EXIT_BOOTSTRAP);
      return;
    }
    if (System.currentTimeMillis() > boot.getExpiresUnixMs()) {
      System.err.println("bootstrap rejected: expired");
      System.exit(EXIT_EXPIRED);
      return;
    }
    int code;
    try {
      code = new Session(boot).run();
    } catch (Exception e) {
      System.err.println("worker error: " + e.getClass().getName() + ": " + e.getMessage());
      code = EXIT_ERROR;
    }
    Timing.mark("exit_call");
    System.exit(code);
  }

  /**
   * Reads one uvarint-length-prefixed Bootstrap message and closes stdin, so
   * the pipe can carry nothing else (INT-02).
   */
  static Bootstrap readBootstrap(InputStream in) throws IOException {
    DataInputStream data = new DataInputStream(in);
    int length = 0;
    for (int shift = 0; ; shift += 7) {
      if (shift > 28) {
        throw new IOException("length prefix too long");
      }
      int b = data.read();
      if (b < 0) {
        throw new IOException("stdin closed before the length prefix");
      }
      length |= (b & 0x7f) << shift;
      if ((b & 0x80) == 0) {
        break;
      }
    }
    if (length <= 0 || length > MAX_BOOTSTRAP) {
      throw new IOException("bootstrap length " + length + " outside 1.." + MAX_BOOTSTRAP);
    }
    byte[] body = new byte[length];
    data.readFully(body);
    in.close();
    return Bootstrap.parseFrom(body);
  }

  static String proof(Bootstrap boot) {
    try {
      Mac mac = Mac.getInstance("HmacSHA256");
      mac.init(new SecretKeySpec(boot.getCredential().toByteArray(), "HmacSHA256"));
      mac.update(PROOF_LABEL.getBytes(StandardCharsets.UTF_8));
      mac.update((byte) 0);
      mac.update(boot.getOperationId().getBytes(StandardCharsets.UTF_8));
      mac.update((byte) 0);
      mac.update(boot.getWorkerId().getBytes(StandardCharsets.UTF_8));
      return HexFormat.of().formatHex(mac.doFinal());
    } catch (Exception e) {
      throw new IllegalStateException("HMAC unavailable", e);
    }
  }

  /** Builds the deterministic payload for record {@code index}. */
  static byte[] payload(long index, int size) {
    byte[] p = new byte[size];
    for (int j = 0; j < size; j++) {
      p[j] = (byte) ((index * 31 + j) & 0xff);
    }
    return p;
  }

  static MessageDigest sha256() {
    try {
      return MessageDigest.getInstance("SHA-256");
    } catch (NoSuchAlgorithmException e) {
      throw new IllegalStateException(e);
    }
  }

  /** One connection and one stream; all callbacks run on one thread. */
  static final class Session implements ClientResponseObserver<WorkerEvent, HostCommand> {
    private final Bootstrap boot;
    private final ExecutorService serial = Executors.newSingleThreadExecutor(r -> {
      Thread t = new Thread(r, "g0-serial");
      t.setDaemon(true);
      return t;
    });
    private final CountDownLatch done = new CountDownLatch(1);
    private final AtomicInteger exitCode = new AtomicInteger(EXIT_ERROR);
    private final MessageDigest sourceDigest = sha256();
    private final MessageDigest recordDigest = sha256();
    private ClientCallStreamObserver<WorkerEvent> out;
    private long sequence;
    private long recordsWanted;
    private int recordBytes;
    private long recordsSent;
    private long sourceBytes;
    private boolean sourceDone;
    private boolean completedSent;
    private long notReadyWaits;
    private long maxHeapUsed;

    Session(Bootstrap boot) {
      this.boot = boot;
    }

    int run() throws Exception {
      Transport t = Transport.open(boot.getTransport(), boot.getEndpoint());
      Metadata headers = new Metadata();
      headers.put(PROOF_KEY, proof(boot));
      // Local IPC has no host name; a fixed authority keeps HTTP/2 valid.
      ManagedChannel channel = t.builder
          .executor(serial)
          .overrideAuthority("localhost")
          .usePlaintext()
          .intercept(MetadataUtils.newAttachHeadersInterceptor(headers))
          .build();
      try {
        WorkerSessionGrpc.newStub(channel).connect(this);
        if (!done.await(15, TimeUnit.MINUTES)) {
          System.err.println("worker: session timed out");
          return EXIT_ERROR;
        }
        Timing.mark("main_wake");
        return exitCode.get();
      } finally {
        channel.shutdownNow();
        Timing.mark("channel_shutdown_now");
        boolean terminated = channel.awaitTermination(5, TimeUnit.SECONDS);
        Timing.mark(terminated ? "channel_terminated" : "channel_not_terminated");
        t.close();
        Timing.mark("transport_closed");
      }
    }

    @Override
    public void beforeStart(ClientCallStreamObserver<WorkerEvent> requestStream) {
      out = requestStream;
      out.setOnReadyHandler(this::pump);
    }

    private void send(WorkerEvent.Builder event) {
      out.onNext(event.setSequence(++sequence).build());
    }

    private void sampleHeap() {
      Runtime rt = Runtime.getRuntime();
      long used = rt.totalMemory() - rt.freeMemory();
      if (used > maxHeapUsed) {
        maxHeapUsed = used;
      }
    }

    /** Sends the hello once the stream is ready, then streams records. */
    private boolean helloSent;

    private void pump() {
      if (!helloSent) {
        helloSent = true;
        List<String> names = new ArrayList<>(System.getenv().keySet());
        names.sort(null);
        send(WorkerEvent.newBuilder().setHello(WorkerHello.newBuilder()
            .setWorkerId(boot.getWorkerId())
            .setProtocolMajor(1)
            .setProtocolMinor(0)
            .setPid(ProcessHandle.current().pid())
            .setJavaVersion(System.getProperty("java.runtime.version", "?"))
            .setTransport(boot.getTransport())
            .addAllEnvNames(names)));
      }
      while (recordsSent < recordsWanted) {
        if (!out.isReady()) {
          notReadyWaits++;
          return; // Flow control: wait for the next onReady callback.
        }
        byte[] p = payload(recordsSent, recordBytes);
        recordDigest.update(ByteBuffer.allocate(8).putLong(recordsSent).array());
        recordDigest.update(p);
        send(WorkerEvent.newBuilder().setRecord(Record.newBuilder()
            .setIndex(recordsSent)
            .setPayload(ByteString.copyFrom(p))));
        recordsSent++;
        if ((recordsSent & 1023) == 0) {
          sampleHeap();
        }
      }
      maybeComplete();
    }

    private void maybeComplete() {
      if (completedSent || !sourceDone || recordsSent < recordsWanted) {
        return;
      }
      completedSent = true;
      sampleHeap();
      send(WorkerEvent.newBuilder().setCompleted(Completed.newBuilder()
          .setRecordsSent(recordsSent)
          .setSourceBytes(sourceBytes)
          .setSourceSha256(ByteString.copyFrom(sourceDigest.digest()))
          .setRecordsSha256(ByteString.copyFrom(recordDigest.digest()))
          .setNotReadyWaits(notReadyWaits)
          .setMaxHeapUsedBytes(maxHeapUsed)));
    }

    @Override
    public void onNext(HostCommand cmd) {
      switch (cmd.getCommandCase()) {
        case PING -> send(WorkerEvent.newBuilder().setPong(Pong.newBuilder().setSeq(cmd.getPing().getSeq())));
        case EXECUTE -> {
          recordsWanted = cmd.getExecute().getRecords();
          recordBytes = cmd.getExecute().getRecordBytes();
          if ("noise".equals(cmd.getExecute().getScenario())) {
            makeNoise();
          }
          if ("reconnect".equals(cmd.getExecute().getScenario())) {
            replayProof();
          }
          pump();
        }
        case SOURCE -> {
          ByteString chunk = cmd.getSource().getChunk();
          sourceDigest.update(chunk.asReadOnlyByteBuffer());
          sourceBytes += chunk.size();
          if (cmd.getSource().getLast()) {
            sourceDone = true;
            maybeComplete();
          }
        }
        case FINISH -> out.onCompleted();
        default -> { }
      }
    }

    /**
     * Opens a second connection from this same process with the same proof
     * and reports how the host answered (TPT-007). The host must refuse it.
     */
    private void replayProof() {
      Thread t = new Thread(() -> {
        String result;
        try (Transport t2 = Transport.open(boot.getTransport(), boot.getEndpoint())) {
          Metadata h = new Metadata();
          h.put(PROOF_KEY, proof(boot));
          ManagedChannel ch2 = t2.builder
              .overrideAuthority("localhost")
              .usePlaintext()
              .intercept(MetadataUtils.newAttachHeadersInterceptor(h))
              .build();
          CountDownLatch answered = new CountDownLatch(1);
          String[] code = {"NO_ANSWER"};
          StreamObserver<WorkerEvent> req = WorkerSessionGrpc.newStub(ch2).connect(new StreamObserver<HostCommand>() {
            @Override
            public void onNext(HostCommand value) {
              code[0] = "ACCEPTED";
              answered.countDown();
            }

            @Override
            public void onError(Throwable e) {
              code[0] = Status.fromThrowable(e).getCode().name();
              answered.countDown();
            }

            @Override
            public void onCompleted() {
              code[0] = "COMPLETED";
              answered.countDown();
            }
          });
          req.onNext(WorkerEvent.newBuilder().setSequence(1)
              .setHello(WorkerHello.newBuilder().setWorkerId("replay")).build());
          answered.await(10, TimeUnit.SECONDS);
          result = "replay:" + code[0];
          ch2.shutdownNow();
        } catch (Exception e) {
          result = "replay:ERROR " + e.getClass().getSimpleName();
        }
        String report = result;
        serial.execute(() -> send(WorkerEvent.newBuilder().setRecord(Record.newBuilder()
            .setIndex(-1L)
            .setPayload(ByteString.copyFromUtf8(report)))));
      }, "g0-replay");
      t.setDaemon(true);
      t.start();
    }

    /** Simulates a chatty library: 1 MiB on stdout and on stderr (TPT-021). */
    private static void makeNoise() {
      String line = "noise ".repeat(170) + "\n"; // about 1 KiB
      PrintStream stdout = System.out;
      for (int i = 0; i < 1024; i++) {
        stdout.print(line);
        System.err.print(line);
      }
      stdout.flush();
      System.err.flush();
    }

    @Override
    public void onError(Throwable t) {
      Timing.mark("stream_error");
      Status s = Status.fromThrowable(t);
      switch (s.getCode()) {
        case CANCELLED -> exitCode.set(EXIT_CANCELLED);
        case UNAUTHENTICATED, PERMISSION_DENIED -> exitCode.set(EXIT_REJECTED);
        default -> exitCode.set(EXIT_ERROR);
      }
      System.err.println("worker: stream ended with " + s.getCode() + ": " + s.getDescription());
      done.countDown();
    }

    @Override
    public void onCompleted() {
      Timing.mark("stream_completed");
      exitCode.set(EXIT_OK);
      done.countDown();
    }
  }

  /** A channel builder for one transport, and what it must release. */
  static final class Transport implements AutoCloseable {
    final NettyChannelBuilder builder;
    private final EventLoopGroup group;
    private final AutoCloseable extra;

    private Transport(NettyChannelBuilder builder, EventLoopGroup group, AutoCloseable extra) {
      this.builder = builder;
      this.group = group;
      this.extra = extra;
    }

    @SuppressWarnings("deprecation")
    static Transport open(String kind, String endpoint) throws Exception {
      switch (kind) {
        case "uds-epoll": {
          EventLoopGroup g = new io.netty.channel.epoll.EpollEventLoopGroup(1);
          return new Transport(NettyChannelBuilder.forAddress(new DomainSocketAddress(endpoint))
              .channelType(io.netty.channel.epoll.EpollDomainSocketChannel.class, DomainSocketAddress.class)
              .eventLoopGroup(g), g, null);
        }
        case "uds-kqueue": {
          EventLoopGroup g = new io.netty.channel.kqueue.KQueueEventLoopGroup(1);
          return new Transport(NettyChannelBuilder.forAddress(new DomainSocketAddress(endpoint))
              .channelType(io.netty.channel.kqueue.KQueueDomainSocketChannel.class, DomainSocketAddress.class)
              .eventLoopGroup(g), g, null);
        }
        case "uds-nio": {
          EventLoopGroup g = new io.netty.channel.nio.NioEventLoopGroup(1);
          return new Transport(NettyChannelBuilder.forAddress(UnixDomainSocketAddress.of(endpoint))
              .channelType(io.netty.channel.socket.nio.NioDomainSocketChannel.class, UnixDomainSocketAddress.class)
              .eventLoopGroup(g), g, null);
        }
        case "npipe":
        case "npipe-dual":
        case "npipe-async": {
          PipeBridge.Mode mode = switch (kind) {
            case "npipe-dual" -> PipeBridge.Mode.SYNC_DUAL;
            case "npipe-async" -> PipeBridge.Mode.ASYNC_SINGLE;
            default -> PipeBridge.Mode.SYNC_SINGLE;
          };
          PipeBridge bridge = PipeBridge.start(endpoint, mode);
          return new Transport(NettyChannelBuilder.forAddress(bridge.address())
              .channelType(io.netty.channel.local.LocalChannel.class, io.netty.channel.local.LocalAddress.class)
              .eventLoopGroup(bridge.group()), null, bridge);
        }
        default:
          throw new IllegalArgumentException("unknown transport " + kind);
      }
    }

    @Override
    public void close() throws Exception {
      if (group != null) {
        group.shutdownGracefully(0, 1, TimeUnit.SECONDS);
      }
      if (extra != null) {
        extra.close();
      }
    }
  }
}
