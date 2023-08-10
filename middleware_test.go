package jaeger_middleware

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/credentials/insecure"
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

func TestMiddlewareServer(t *testing.T) {
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
	var addr string
	addr = ":50055"
	ctx := context.WithValue(context.Background(), "trace-id", "6501471c16ed88eef15152f1930e2af4")
	var req1 = &proto.GetReq{
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
	resp, err := client.Get(ctx, req1)
	if err != nil {
		log.Fatal(err)
	}
	assert.Equal(t, req1.GetName(), resp.GetName())
}

func TestTraceID(t *testing.T) {
	g := middleware.JaegerIDGenerator{}
	tt, _ := g.NewIDs(context.Background())
	println(tt.String())
}
