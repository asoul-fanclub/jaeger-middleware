package jaeger_middleware

import (
	"context"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"google.golang.org/grpc"
	"jaeger-middleware/middleware"
	"jaeger-middleware/test"
	"jaeger-middleware/test/proto"
)

var (
	service     = "trace-demo"
	environment = "production"
	id          = 1
)

func TestMiddleware(t *testing.T) {
	tp, _ := tracerProvider("http://localhost:14268/api/traces")
	otel.SetTracerProvider(tp)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
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

// tracerProvider returns an OpenTelemetry TracerProvider configured to use
// the Jaeger exporter that will send spans to the provided url. The returned
// TracerProvider will also use a Resource configured with all the information
// about the application.
func tracerProvider(url string) (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}
	var env string = environment
	if os.Getenv("environment") != "" {
		env = os.Getenv("environment")
	}
	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(service),
			attribute.String("environment", env),
			attribute.Int64("ID", int64(id)),
		)),
	)
	return tp, nil
}
