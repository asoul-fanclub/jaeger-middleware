package middleware

import (
	"context"
	"encoding/json"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

type JaegerMiddleware struct{}

func NewJaegerMiddleware() *JaegerMiddleware {
	return &JaegerMiddleware{}
}

// UnaryInterceptor TODO: one method will get one child span or controlled by LogWithContext
// a service call
func (jm *JaegerMiddleware) UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	o := DefaultOptions()
	newCtx, span := o.tracer.Start(ctx, info.FullMethod)
	defer span.End()

	if body, _ := json.Marshal(req); len(body) > 0 {
		if len(body) > o.maxBodySize {
			body = []byte(`input body too large`)
		}
		span.SetAttributes(attribute.String(inputKey, string(body)))
	}

	resp, err := handler(newCtx, req)
	defer finishServerSpan(span, err)
	return resp, err
}

func (jm *JaegerMiddleware) StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return nil
}

func finishServerSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, codes.Ok.String())
	}
}
