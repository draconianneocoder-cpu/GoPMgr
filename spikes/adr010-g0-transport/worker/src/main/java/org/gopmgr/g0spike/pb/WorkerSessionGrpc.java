package org.gopmgr.g0spike.pb;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * The Go host is the gRPC server; the worker connects as the client and
 * opens one bidirectional stream (plan §2.2).
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class WorkerSessionGrpc {

  private WorkerSessionGrpc() {}

  public static final java.lang.String SERVICE_NAME = "gopmgr.g0spike.v1.WorkerSession";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<org.gopmgr.g0spike.pb.WorkerEvent,
      org.gopmgr.g0spike.pb.HostCommand> getConnectMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Connect",
      requestType = org.gopmgr.g0spike.pb.WorkerEvent.class,
      responseType = org.gopmgr.g0spike.pb.HostCommand.class,
      methodType = io.grpc.MethodDescriptor.MethodType.BIDI_STREAMING)
  public static io.grpc.MethodDescriptor<org.gopmgr.g0spike.pb.WorkerEvent,
      org.gopmgr.g0spike.pb.HostCommand> getConnectMethod() {
    io.grpc.MethodDescriptor<org.gopmgr.g0spike.pb.WorkerEvent, org.gopmgr.g0spike.pb.HostCommand> getConnectMethod;
    if ((getConnectMethod = WorkerSessionGrpc.getConnectMethod) == null) {
      synchronized (WorkerSessionGrpc.class) {
        if ((getConnectMethod = WorkerSessionGrpc.getConnectMethod) == null) {
          WorkerSessionGrpc.getConnectMethod = getConnectMethod =
              io.grpc.MethodDescriptor.<org.gopmgr.g0spike.pb.WorkerEvent, org.gopmgr.g0spike.pb.HostCommand>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.BIDI_STREAMING)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Connect"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.gopmgr.g0spike.pb.WorkerEvent.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  org.gopmgr.g0spike.pb.HostCommand.getDefaultInstance()))
              .setSchemaDescriptor(new WorkerSessionMethodDescriptorSupplier("Connect"))
              .build();
        }
      }
    }
    return getConnectMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static WorkerSessionStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<WorkerSessionStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<WorkerSessionStub>() {
        @java.lang.Override
        public WorkerSessionStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new WorkerSessionStub(channel, callOptions);
        }
      };
    return WorkerSessionStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static WorkerSessionBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<WorkerSessionBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<WorkerSessionBlockingV2Stub>() {
        @java.lang.Override
        public WorkerSessionBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new WorkerSessionBlockingV2Stub(channel, callOptions);
        }
      };
    return WorkerSessionBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static WorkerSessionBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<WorkerSessionBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<WorkerSessionBlockingStub>() {
        @java.lang.Override
        public WorkerSessionBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new WorkerSessionBlockingStub(channel, callOptions);
        }
      };
    return WorkerSessionBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static WorkerSessionFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<WorkerSessionFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<WorkerSessionFutureStub>() {
        @java.lang.Override
        public WorkerSessionFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new WorkerSessionFutureStub(channel, callOptions);
        }
      };
    return WorkerSessionFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * The Go host is the gRPC server; the worker connects as the client and
   * opens one bidirectional stream (plan §2.2).
   * </pre>
   */
  public interface AsyncService {

    /**
     */
    default io.grpc.stub.StreamObserver<org.gopmgr.g0spike.pb.WorkerEvent> connect(
        io.grpc.stub.StreamObserver<org.gopmgr.g0spike.pb.HostCommand> responseObserver) {
      return io.grpc.stub.ServerCalls.asyncUnimplementedStreamingCall(getConnectMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service WorkerSession.
   * <pre>
   * The Go host is the gRPC server; the worker connects as the client and
   * opens one bidirectional stream (plan §2.2).
   * </pre>
   */
  public static abstract class WorkerSessionImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return WorkerSessionGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service WorkerSession.
   * <pre>
   * The Go host is the gRPC server; the worker connects as the client and
   * opens one bidirectional stream (plan §2.2).
   * </pre>
   */
  public static final class WorkerSessionStub
      extends io.grpc.stub.AbstractAsyncStub<WorkerSessionStub> {
    private WorkerSessionStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected WorkerSessionStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new WorkerSessionStub(channel, callOptions);
    }

    /**
     */
    public io.grpc.stub.StreamObserver<org.gopmgr.g0spike.pb.WorkerEvent> connect(
        io.grpc.stub.StreamObserver<org.gopmgr.g0spike.pb.HostCommand> responseObserver) {
      return io.grpc.stub.ClientCalls.asyncBidiStreamingCall(
          getChannel().newCall(getConnectMethod(), getCallOptions()), responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service WorkerSession.
   * <pre>
   * The Go host is the gRPC server; the worker connects as the client and
   * opens one bidirectional stream (plan §2.2).
   * </pre>
   */
  public static final class WorkerSessionBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<WorkerSessionBlockingV2Stub> {
    private WorkerSessionBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected WorkerSessionBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new WorkerSessionBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    @io.grpc.ExperimentalApi("https://github.com/grpc/grpc-java/issues/10918")
    public io.grpc.stub.BlockingClientCall<org.gopmgr.g0spike.pb.WorkerEvent, org.gopmgr.g0spike.pb.HostCommand>
        connect() {
      return io.grpc.stub.ClientCalls.blockingBidiStreamingCall(
          getChannel(), getConnectMethod(), getCallOptions());
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service WorkerSession.
   * <pre>
   * The Go host is the gRPC server; the worker connects as the client and
   * opens one bidirectional stream (plan §2.2).
   * </pre>
   */
  public static final class WorkerSessionBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<WorkerSessionBlockingStub> {
    private WorkerSessionBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected WorkerSessionBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new WorkerSessionBlockingStub(channel, callOptions);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service WorkerSession.
   * <pre>
   * The Go host is the gRPC server; the worker connects as the client and
   * opens one bidirectional stream (plan §2.2).
   * </pre>
   */
  public static final class WorkerSessionFutureStub
      extends io.grpc.stub.AbstractFutureStub<WorkerSessionFutureStub> {
    private WorkerSessionFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected WorkerSessionFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new WorkerSessionFutureStub(channel, callOptions);
    }
  }

  private static final int METHODID_CONNECT = 0;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final AsyncService serviceImpl;
    private final int methodId;

    MethodHandlers(AsyncService serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_CONNECT:
          return (io.grpc.stub.StreamObserver<Req>) serviceImpl.connect(
              (io.grpc.stub.StreamObserver<org.gopmgr.g0spike.pb.HostCommand>) responseObserver);
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getConnectMethod(),
          io.grpc.stub.ServerCalls.asyncBidiStreamingCall(
            new MethodHandlers<
              org.gopmgr.g0spike.pb.WorkerEvent,
              org.gopmgr.g0spike.pb.HostCommand>(
                service, METHODID_CONNECT)))
        .build();
  }

  private static abstract class WorkerSessionBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    WorkerSessionBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return org.gopmgr.g0spike.pb.G0Spike.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("WorkerSession");
    }
  }

  private static final class WorkerSessionFileDescriptorSupplier
      extends WorkerSessionBaseDescriptorSupplier {
    WorkerSessionFileDescriptorSupplier() {}
  }

  private static final class WorkerSessionMethodDescriptorSupplier
      extends WorkerSessionBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    WorkerSessionMethodDescriptorSupplier(java.lang.String methodName) {
      this.methodName = methodName;
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.MethodDescriptor getMethodDescriptor() {
      return getServiceDescriptor().findMethodByName(methodName);
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (WorkerSessionGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new WorkerSessionFileDescriptorSupplier())
              .addMethod(getConnectMethod())
              .build();
        }
      }
    }
    return result;
  }
}
