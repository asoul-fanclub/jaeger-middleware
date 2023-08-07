package middleware

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel/codes"
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

	resp, err := handler(newCtx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	fmt.Println(resp)
	return resp, err
}

func (jm *JaegerMiddleware) StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return nil
}
