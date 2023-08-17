package jaeger_middleware

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"jaeger-middleware/middleware"
	"jaeger-middleware/test"
	"jaeger-middleware/test/proto"
)

func TestMiddlewareServer(t *testing.T) {
	tp, _ := middleware.TracerProvider("http://localhost:14268/api/traces", false)
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
		grpc.UnaryInterceptor(middleware.NewJaegerServerMiddleware().UnaryInterceptor),
		grpc.StreamInterceptor(middleware.NewJaegerServerMiddleware().StreamInterceptor),
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

func TestMiddlewareClient(t *testing.T) {
	tp, _ := middleware.TracerProvider("http://localhost:14268/api/traces", false)
	otel.SetTracerProvider(tp)
	var addr string
	addr = ":50055"
	ctx := context.WithValue(context.Background(), "trace-id", "cdde169b504ec847521a2cf1d1ffa9f9")
	req := &proto.GetReq{
		Name: "www3",
	}
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(middleware.NewJaegerClientMiddleware().UnaryClientInterceptor),
		grpc.WithStreamInterceptor(middleware.NewJaegerClientMiddleware().StreamClientInterceptor),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := proto.NewTestServiceClient(conn)
	resp, err := client.Get(ctx, req)
	assert.Nil(t, err)
	assert.Equal(t, "www", resp.GetName())
}

func TestTraceID(t *testing.T) {
	g := middleware.JaegerIDGenerator{}
	tt, _ := g.NewIDs(context.Background())
	println(tt.String())
}
