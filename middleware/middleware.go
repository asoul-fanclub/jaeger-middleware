package middleware

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
)

type JaegerMiddleware struct{}

func NewJaegerMiddleware() *JaegerMiddleware {
	return &JaegerMiddleware{}
}

func (jm *JaegerMiddleware) UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	o := DefaultOptions()
	newCtx, span := o.tracer.Start(ctx, info.FullMethod)
	resp, err := handler(newCtx, req)
	if err != nil {
		span.End()
		return nil, err
	}
	fmt.Println(resp)
	span.End()
	return resp, err
}

func (jm *JaegerMiddleware) StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return nil
}
