package jaeger_middleware

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"jaeger-middleware/middleware"
	"jaeger-middleware/test"
	"jaeger-middleware/test/proto"
)

func TestMiddleware(t *testing.T) {
	tp, _ := middleware.TracerProvider("http://localhost:14268/api/traces")
	otel.SetTracerProvider(tp)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			fmt.Println("failed to shutdown TracerProvider: ", err)
		}
	}(ctx)

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

func TestTraceID(t *testing.T) {
	g := middleware.JaegerIDGenerator{}
	tt, _ := g.NewIDs(context.Background())
	println(tt.String())
}
