package middleware

import (
	"context"
	cRand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/rand"
	"sync"
	"time"

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

// UnaryInterceptor
// a service call
func (jsm *JaegerServerMiddleware) UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	o := DefaultOptions()

	// Get "trace-id" from the request header
	traceID, _ := GetTraceIDFromHeader(ctx)
	if traceID.IsValid() {
		ctx = context.WithValue(ctx, "trace-id", traceID.String())
	}

	newCtx, span := newServerSpan(ctx, o.tracer, info.FullMethod)
	defer span.End(trace.WithTimestamp(time.Now()))

	if !traceID.IsValid() {
		newCtx = context.WithValue(newCtx, "trace-id", span.SpanContext().TraceID().String())
	}
	if body, _ := json.Marshal(req); len(body) > 0 {
		if len(body) > o.maxBodySize {
			body = []byte(`input body too large`)
		}
		span.SetAttributes(attribute.String(inputKey, string(body)))
	}

	span.SetAttributes(semconv.NetSockPeerAddrKey.String(Addr(ctx)))
	resp, err := handler(newCtx, req)
	finishServerSpan(span, err, o.maxBodySize)
	return resp, err
}

func (jsm *JaegerServerMiddleware) StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return nil
}

func newServerSpan(ctx context.Context, tracer trace.Tracer, spanName string) (context.Context, trace.Span) {
	newCtx, span := tracer.Start(
		ctx, spanName,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithTimestamp(time.Now()),
	)
	return trace.ContextWithSpan(newCtx, span), span
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

// ---------------------------- Client ----------------------------

type JaegerClientMiddleware struct{}

func NewJaegerClientMiddleware() *JaegerClientMiddleware {
	return &JaegerClientMiddleware{}
}

func (jcm *JaegerClientMiddleware) UnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	o := DefaultOptions()
	var handlerErr error

	newCtx, span := newClientSpan(ctx, o.tracer, method)
	defer span.End(trace.WithTimestamp(time.Now()))
	if body, _ := json.Marshal(req); len(body) > 0 {
		if len(body) > o.maxBodySize {
			body = []byte(`input body too large`)
		}
		span.SetAttributes(attribute.String(inputKey, string(body)))
	}

	if traceIDStr, ok := newCtx.Value("trace-id").(string); ok && traceIDStr != "" {
		md := metadata.Pairs(TraceHeader(), traceIDStr)
		newCtx = metadata.NewOutgoingContext(newCtx, md)
	}
	handlerErr = invoker(newCtx, method, req, reply, cc, opts...)
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
	newCtx, span := tracer.Start(
		ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithTimestamp(time.Now()),
	)
	return trace.ContextWithSpan(newCtx, span), span
}

// GetTraceIDFromHeader tries to extract the trace ID from the request header.
func GetTraceIDFromHeader(ctx context.Context) (trace.TraceID, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return trace.TraceID{}, errors.New("failed to get metadata from context")
	}
	traceIDs := md.Get("trace-id")
	if len(traceIDs) > 0 {
		traceIDStr := traceIDs[0]
		traceID, err := trace.TraceIDFromHex(traceIDStr)
		if err != nil {
			return trace.TraceID{}, err
		}
		return traceID, nil
	}
	return trace.TraceID{}, nil
}

// ---------------------------- IDGenerator ----------------------------

type JaegerIDGenerator struct {
	traceID string
	spanID  string
	sync.Mutex
	randSource *rand.Rand
}

func (gen *JaegerIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	if gen.randSource == nil {
		gen.defaultIDGenerator()
	}
	if ctx.Value("trace-id") != "" {
		str, ok := ctx.Value("trace-id").(string)
		if !ok {
			t, s := gen.newRandIDs()
			ctx = context.WithValue(ctx, "trace-id", t.String())
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
	if err != nil {
		return nil, err
	}
	var tp *traceSdk.TracerProvider
	meta := GetMetaData()
	if withK8SSource {
		tp = traceSdk.NewTracerProvider(
			// Always be sure to batch in production.
			traceSdk.WithBatcher(exp),
			traceSdk.WithIDGenerator(&JaegerIDGenerator{}),
			// Record information about this application in a Resource.
			traceSdk.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.K8SPodName(meta.PodName),
				semconv.K8SClusterName(meta.ClusterName),
				semconv.K8SDeploymentName(meta.Deployment),
				semconv.K8SNamespaceName(meta.Namespace),
				semconv.HostName(meta.HostName),
				semconv.ServiceName(SName()),
				semconv.DeploymentEnvironment(Env()),
			)),
		)
	} else {
		tp = traceSdk.NewTracerProvider(
			traceSdk.WithBatcher(exp),
			traceSdk.WithIDGenerator(&JaegerIDGenerator{}),
			traceSdk.WithResource(resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.HostName(meta.HostName),
				semconv.ServiceName(SName()),
				semconv.DeploymentEnvironment(Env()),
			)),
		)
	}

	return tp, nil
}
