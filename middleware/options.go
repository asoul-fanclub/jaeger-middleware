package middleware

import (
	"context"
	"google.golang.org/grpc/metadata"
	"os"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/peer"
)

const (
	defaultInputHeader   = "input_param"
	defaultEnvironment   = "develop"
	defaultServiceName   = "default_service_name"
	defaultCluster       = "default_cluster"
	defaultNameSpace     = "default_namespace"
	defaultDeployment    = "default_deployment"
	defaultHostName      = "default_hostname"
	defaultPodName       = "default_pod"
	defaultTraceIDHeader = "trace-id"
	CurrentSpanContext   = "current-span-context"
)

type traceIdType int

const currentTrace traceIdType = iota

var (
	optionOnce sync.Once
	o          Options
)

type JaegerMiddleOptionFunc func(*Options)

type Options struct {
	tracer        trace.Tracer
	maxBodySize   int
	serverEnabled bool
	clientEnabled bool
	meta          MetaData
}

// DefaultOptions
// 优先级：自主设置->环境变量->默认设置
func DefaultOptions() Options {
	optionOnce.Do(func() {
		o = Options{}
		o.meta = MetaData{
			ClusterName: defaultCluster,
			Namespace:   defaultNameSpace,
			Deployment:  defaultDeployment,
			HostName:    defaultHostName,
			PodName:     defaultPodName,
			ServiceName: defaultServiceName,
			Environment: defaultEnvironment,
			TraceHeader: defaultTraceIDHeader,
			InputHeader: defaultInputHeader,
		}
		if os.Getenv("CLUSTER_NAME") != "" {
			o.meta.ClusterName = os.Getenv("CLUSTER_NAME")
		}
		if os.Getenv("NAMESPACE") != "" {
			o.meta.Namespace = os.Getenv("NAMESPACE")
		}
		if os.Getenv("DEPLOYMENT") != "" {
			o.meta.Deployment = os.Getenv("DEPLOYMENT")
		}
		if hostname, err := os.Hostname(); err == nil {
			o.meta.HostName = strings.TrimSpace(hostname)
			o.meta.PodName = strings.TrimSpace(hostname)
		}
		if os.Getenv("SERVICE_NAME") != "" {
			o.meta.ServiceName = os.Getenv("SERVICE_NAME")
		}
		if os.Getenv("ENVIRONMENT") != "" {
			o.meta.Environment = os.Getenv("ENVIRONMENT")
		}
		if os.Getenv("TRACE_HEADER") != "" {
			o.meta.TraceHeader = os.Getenv("TRACE_HEADER")
		}

		o.maxBodySize = 10240
		o.serverEnabled = true
		o.clientEnabled = true
		if maxBoxSizeStr := os.Getenv("MAX_BOX_SIZE"); len(maxBoxSizeStr) > 0 {
			maxBodySize, err := strconv.Atoi(maxBoxSizeStr)
			if err == nil {
				o.maxBodySize = maxBodySize
			}
		}
		if serverEnabled := os.Getenv("SERVER_ENABLED"); strings.ToUpper(strings.TrimSpace(serverEnabled)) == "FALSE" {
			o.serverEnabled = false
		}
		if clientEnabled := os.Getenv("CLIENT_ENABLED"); strings.ToUpper(strings.TrimSpace(clientEnabled)) == "FALSE" {
			o.clientEnabled = false
		}
	})
	return o
}

type MetaData struct {
	ClusterName string `json:"cluster_name"`
	Namespace   string `json:"namespace"`
	Deployment  string `json:"deployment"`
	PodName     string `json:"pod_name"`
	HostName    string `json:"hostname"`
	ServiceName string `json:"service_name"`
	Environment string `json:"environment"`
	TraceHeader string `json:"trace_header"`
	InputHeader string `json:"input_header"`
}

func Addr(ctx context.Context) (addr string) {
	p, _ := peer.FromContext(ctx)
	if p != nil {
		return p.Addr.String()
	}
	return "unknown peer addr"
}

func inject(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}
	md.Append(o.meta.TraceHeader, trace.SpanFromContext(ctx).SpanContext().TraceID().String())
	md.Append(CurrentSpanContext, trace.SpanFromContext(ctx).SpanContext().SpanID().String())
	return metadata.NewOutgoingContext(ctx, md)
}

func extract(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	res := trace.SpanContextConfig{}

	o := DefaultOptions()
	var err error
	traceIDs := md.Get(o.meta.TraceHeader)
	curSpanContext := md.Get(CurrentSpanContext)
	if len(traceIDs) > 0 {
		res.TraceID, err = trace.TraceIDFromHex(traceIDs[0])
		if err != nil {
			return ctx
		}
		if len(curSpanContext) > 0 {
			spanID, err := trace.SpanIDFromHex(curSpanContext[0])
			if err != nil {
				return ctx
			}
			res.SpanID = spanID
		}
		return trace.ContextWithRemoteSpanContext(ctx, trace.NewSpanContext(res))
	}
	return ctx
}

func SetTraceID(ctx context.Context, traceId string) context.Context {
	return context.WithValue(ctx, currentTrace, traceId)
}
