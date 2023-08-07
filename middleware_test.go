package jaeger_middleware

import (
	"google.golang.org/grpc"
	"jaeger-middleware/middleware"
	"jaeger-middleware/test"
	"jaeger-middleware/test/proto"
	"log"
	"net"
	"testing"
)

func TestMiddleware(t *testing.T) {
	server := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.NewJaegerMiddleware().UnaryInterceptor),
		grpc.StreamInterceptor(middleware.NewJaegerMiddleware().StreamInterceptor),
	)
	proto.RegisterTestServiceServer(server, new(test.UserService))
	list, err := net.Listen("tcp", ":50055")
	if err != nil {
		log.Fatal(err)
	}
	err = server.Serve(list)
	if err != nil {
		log.Fatal(err)
	}
}
