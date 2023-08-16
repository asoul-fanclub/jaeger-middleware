package middleware

import (
	"context"
	cRand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"math/rand"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	traceSdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ---------------------------- Server ----------------------------

type JaegerServerMiddleware struct{}

func NewJaegerServerMiddleware() *JaegerServerMiddleware {
	return &JaegerServerMiddleware{}
}

// UnaryInterceptor a service call
func (jsm *JaegerServerMiddleware) UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	o := DefaultOptions()
	tracer := otel.Tracer(o.meta.ServiceName)
	if !o.serverEnabled {
		return handler(ctx, req)
	}
	ctx = extract(ctx)
	var span trace.Span
	// Get DefaultTraceIDHeader from the request header
	if trace.SpanContextFromContext(ctx).IsValid() {
		ctx = context.WithValue(ctx, o.meta.TraceHeader, trace.SpanContextFromContext(ctx).TraceID().String())
	}
	ctx, span = newServerSpan(ctx, tracer, info.FullMethod)
	defer span.End()

	if !trace.SpanContextFromContext(ctx).TraceID().IsValid() {
		ctx = context.WithValue(ctx, o.meta.TraceHeader, span.SpanContext().TraceID().String())
	}
	if body, _ := json.Marshal(req); len(body) > 0 {
		if len(body) > o.maxBodySize {
			body = []byte(`input body too large`)
		}
		span.SetAttributes(attribute.String(o.meta.InputHeader, string(body)))
	}
	span.SetAttributes(semconv.NetSockPeerAddrKey.String(Addr(ctx)))
	resp, err := handler(ctx, req)
	finishServerSpan(span, err, o.maxBodySize)
	return resp, err
}

func (jsm *JaegerServerMiddleware) StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return nil
}

func newServerSpan(ctx context.Context, tracer trace.Tracer, spanName string) (context.Context, trace.Span) {
	return tracer.Start(
		trace.ContextWithRemoteSpanContext(ctx, trace.SpanContextFromContext(ctx)), spanName,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithTimestamp(time.Now()),
	)
}

func finishServerSpan(span trace.Span, err error, bodySize int) {
	if err != nil {
		span.RecordError(err, trace.WithTimestamp(time.Now()))
		var errMsg string
		if body, _ := json.Marshal(err); len(body) > 0 {
			if len(body) > bodySize {
				errMsg = "err msg is too large"
			} else {
				errMsg = err.Error()
			}
		}
		span.SetStatus(codes.Error, errMsg)
		return
	}
	span.SetStatus(codes.Ok, codes.Ok.String())
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx  context.Context
	span trace.Span
}

func (wss *wrappedServerStream) Context() context.Context {
	return wss.ctx
}

func (wss *wrappedServerStream) SendMsg(m interface{}) error {
	return wss.ServerStream.SendMsg(m)
}

func (wss *wrappedServerStream) RecvMsg(m interface{}) error {
	return wss.ServerStream.RecvMsg(m)
}

// ---------------------------- Client ----------------------------

type JaegerClientMiddleware struct{}

func NewJaegerClientMiddleware() *JaegerClientMiddleware {
	return &JaegerClientMiddleware{}
}

func (jcm *JaegerClientMiddleware) UnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	o := DefaultOptions()
	if !o.clientEnabled {
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	var handlerErr error
	tracer := otel.Tracer(o.meta.ServiceName)
	ctx, span := newClientSpan(ctx, tracer, method)
	defer span.End()

	if body, _ := json.Marshal(req); len(body) > 0 {
		if len(body) > o.maxBodySize {
			body = []byte(`input body too large`)
		}
		span.SetAttributes(attribute.String(o.meta.InputHeader, string(body)))
	}

	if traceIDStr, ok := ctx.Value(o.meta.TraceHeader).(string); ok && traceIDStr != "" {
		md := metadata.Pairs(o.meta.TraceHeader, traceIDStr)
		md.Append(CurrentSpanContext, trace.SpanFromContext(ctx).SpanContext().SpanID().String())
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	handlerErr = invoker(ctx, method, req, reply, cc, opts...)
	finishClientSpan(span, handlerErr, o.maxBodySize)

	return handlerErr
}

func (jcm *JaegerClientMiddleware) StreamClientInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}

func finishClientSpan(span trace.Span, err error, bodySize int) {
	if err != nil {
		span.RecordError(err, trace.WithTimestamp(time.Now()))
		var errMsg string
		if body, _ := json.Marshal(err); len(body) > 0 {
			if len(body) > bodySize {
				errMsg = "err msg is too large"
			} else {
				errMsg = err.Error()
			}
		}
		span.SetStatus(codes.Error, errMsg)
		return
	}
	span.SetStatus(codes.Ok, codes.Ok.String())
}

func newClientSpan(ctx context.Context, tracer trace.Tracer, spanName string) (context.Context, trace.Span) {
	return tracer.Start(
		ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithTimestamp(time.Now()),
	)
}

type wrappedClientStream struct {
	grpc.ClientStream
	ctx  context.Context
	span trace.Span
}

func (wcs *wrappedClientStream) Context() context.Context {
	return wcs.ctx
}

func (wcs *wrappedClientStream) SendMsg(m interface{}) error {
	err := wcs.ClientStream.SendMsg(m)
	if err != nil && wcs.span != nil {
		wcs.span.RecordError(err, trace.WithTimestamp(time.Now()))
	}
	return err
}

func (wcs *wrappedClientStream) RecvMsg(m interface{}) error {
	err := wcs.ClientStream.RecvMsg(m)
	if err != nil && wcs.span != nil {
		wcs.span.RecordError(err, trace.WithTimestamp(time.Now()))
	}
	return err
}

// ---------------------------- IDGenerator ----------------------------

type JaegerIDGenerator struct {
	traceID string
	spanID  string
	sync.Mutex
	randSource *rand.Rand
}

func (gen *JaegerIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	o := DefaultOptions()
	if gen.randSource == nil {
		gen.defaultIDGenerator()
	}
	if ctx.Value(o.meta.TraceHeader) != "" {
		str, ok := ctx.Value(o.meta.TraceHeader).(string)
		if !ok {
			t, s := gen.newRandIDs()
			return t, s
		}
		t, _ := trace.TraceIDFromHex(str)
		return t, gen.newRandSpanID()
	}

	return gen.newRandIDs()
}

func (gen *JaegerIDGenerator) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
	if gen.randSource == nil {
		gen.defaultIDGenerator()
	}
	return gen.newRandSpanID()
}

var _ traceSdk.IDGenerator = (*JaegerIDGenerator)(nil)

func (gen *JaegerIDGenerator) newRandIDs() (trace.TraceID, trace.SpanID) {
	gen.Lock()
	defer gen.Unlock()
	tid := trace.TraceID{}
	_, _ = gen.randSource.Read(tid[:])
	sid := trace.SpanID{}
	_, _ = gen.randSource.Read(sid[:])
	return tid, sid
}

// NewSpanID returns a non-zero span ID from a randomly-chosen sequence.
func (gen *JaegerIDGenerator) newRandSpanID() trace.SpanID {
	gen.Lock()
	defer gen.Unlock()
	sid := trace.SpanID{}
	_, _ = gen.randSource.Read(sid[:])
	return sid
}

func (gen *JaegerIDGenerator) defaultIDGenerator() {
	gen.Lock()
	defer gen.Unlock()
	if gen.randSource == nil {
		var rngSeed int64
		_ = binary.Read(cRand.Reader, binary.LittleEndian, &rngSeed)
		gen.randSource = rand.New(rand.NewSource(rngSeed))
	}
}

// ---------------------------- Provider ----------------------------

// TracerProvider returns an OpenTelemetry TracerProvider configured to use
// the Jaeger exporter that will send spans to the provided url. The returned
// TracerProvider will also use a Resource configured with all the information
// about the application.
func TracerProvider(url string, withK8SSource bool) (*traceSdk.TracerProvider, error) {
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	o := DefaultOptions()
	if err != nil {
		return nil, err
	}
	var tp *traceSdk.TracerProvider
	if withK8SSource {
		tp = traceSdk.NewTracerProvider(
			// Always be sure to batch in production.
			traceSdk.WithBatcher(exp),
			traceSdk.WithIDGenerator(&JaegerIDGenerator{}),
			// Record information about this application in a Resource.
			traceSdk.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.K8SPodName(o.meta.PodName),
				semconv.K8SClusterName(o.meta.ClusterName),
				semconv.K8SDeploymentName(o.meta.Deployment),
				semconv.K8SNamespaceName(o.meta.Namespace),
				semconv.HostName(o.meta.HostName),
				semconv.ServiceName(o.meta.ServiceName),
				semconv.DeploymentEnvironment(o.meta.Environment),
			)),
		)
	} else {
		tp = traceSdk.NewTracerProvider(
			traceSdk.WithBatcher(exp),
			traceSdk.WithIDGenerator(&JaegerIDGenerator{}),
			traceSdk.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.HostName(o.meta.HostName),
				semconv.ServiceName(o.meta.ServiceName),
				semconv.DeploymentEnvironment(o.meta.Environment),
			)),
		)
	}

	return tp, nil
}
