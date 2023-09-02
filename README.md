# jaeger-middleware

An extensively configurable Go web middleware that integrates Jaeger for distributed tracing.

## Usage

### Installation

```bash
go get github.com/asoul-fanclub/jaeger-middleware@v0.0.6
```

### Example

#### Server

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net"
    "time"

    "github.com/asoul-fanclub/jaeger-middleware/middleware"
    "go.opentelemetry.io/otel"
    "google.golang.org/grpc"

    "test_jaeger/test"
    "test_jaeger/test/proto"
)

func main() {
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
```

#### Client
```go
func main() {
	tp, _ := middleware.TracerProvider("http://localhost:14268/api/traces", false)
	otel.SetTracerProvider(tp)
	var addr string
	addr = ":50055"
	ctx := context.Background()
	ctx = middleware.SetTraceID(ctx, "cdde169b504ec847521a2cf1d1ffa9f9")
	req := &proto.GetReq{
		Name: "xxx",
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
}
```

## Configuration

At present, configuration exclusively relies on manipulation of environmental variables within the operating system. Hence, it is advisable to operate a distinct application within an individual container. This approach facilitates the accommodation of multiple applications on a single host, each employing distinct customized configurations.

- CLUSTER_NAME, default default_cluster
- NAMESPACE, default default_namespace
- DEPLOYMENT, default default_deployment
- SERVICE_NAME, default default_hostname
- ENVIRONMENT, default default_pod
- TRACE_HEADER, default trace-id

The identifying key for the trace_id propagated across the chain of communication.

- MAX_BOX_SIZE, default 10240

If the size of the input body or the returned error message exceeds the MAX_BOX_SIZE, it will not be logged.

- SERVER_ENABLED, default true 

If configured as 'false,' the server middleware will remain inactive.

- CLIENT_ENABLED=dev, default true

If configured as 'false,' the client middleware will remain inactive.

## Effect

![img.png](/img/img.png)

## Reference

TODO
