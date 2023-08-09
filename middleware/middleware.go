package middleware

import (
	"context"
	"encoding/json"
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

type JaegerMiddleware struct{}

func NewJaegerMiddleware() *JaegerMiddleware {
	return &JaegerMiddleware{}
}

// UnaryInterceptor TODO: one method will get one child span or controlled by LogWithContext
// a service call
func (jm *JaegerMiddleware) UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	o := DefaultOptions()
	newCtx, span := newServerSpan(ctx, o.tracer, info.FullMethod)
	defer span.End(trace.WithTimestamp(time.Now()))

	if body, _ := json.Marshal(req); len(body) > 0 {
		if len(body) > o.maxBodySize {
			body = []byte(`input body too large`)
		}
		span.SetAttributes(attribute.String(inputKey, string(body)))
	}
	_ = grpc.SetHeader(ctx, metadata.Pairs(TraceHeader(), span.SpanContext().TraceID().String()))
	span.SetAttributes(semconv.NetSockPeerAddrKey.String(Addr(ctx)))
	resp, err := handler(newCtx, req)
	defer finishServerSpan(span, err, o.maxBodySize)
	return resp, err
}

func (jm *JaegerMiddleware) StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return nil
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

func newServerSpan(ctx context.Context, tracer trace.Tracer, spanName string) (context.Context, trace.Span) {
	newCtx, span := tracer.Start(
		ctx, spanName,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithTimestamp(time.Now()),
	)
	return trace.ContextWithSpan(newCtx, span), span
}

////
////type JaegerIDGenerator struct {
////	traceID int
////	spanID  int
////}
////
////func (gen *JaegerIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
////	// TODO: get trace from header
////	traceIDHex := fmt.Sprintf("%032x", gen.traceID)
////	traceID, _ := trace.TraceIDFromHex(traceIDHex)
////	spanID := gen.NewSpanID(ctx, traceID)
////	gen.traceID++
////	return traceID, spanID
////}
////
////func (gen *JaegerIDGenerator) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
////	spanIDHex := fmt.Sprintf("%016x", gen.spanID)
////	spanID, _ := trace.SpanIDFromHex(spanIDHex)
////	gen.spanID++
////	return spanID
////}
//
//var _ traceSdk.IDGenerator = (*JaegerIDGenerator)(nil)

// TracerProvider returns an OpenTelemetry TracerProvider configured to use
// the Jaeger exporter that will send spans to the provided url. The returned
// TracerProvider will also use a Resource configured with all the information
// about the application.
func TracerProvider(url string) (*traceSdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}

	meta := GetMetaData()
	tp := traceSdk.NewTracerProvider(
		// Always be sure to batch in production.
		traceSdk.WithBatcher(exp),
		// traceSdk.WithIDGenerator(&JaegerIDGenerator{}),
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
	return tp, nil
}
