package middleware

import (
	"context"
	"google.golang.org/grpc"
)

type JaegerMiddleware struct{}

func NewJaegerMiddleware() *JaegerMiddleware {
	return &JaegerMiddleware{}
}

func (jm *JaegerMiddleware) UnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return nil, nil
}

func (jm *JaegerMiddleware) StreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return nil
}
