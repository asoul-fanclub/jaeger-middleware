package middleware

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/peer"
)

const (
	DefaultInputHeader = "input_param"
	DefaultEnvironment = "develop"
	DefaultServiceName = "default_service_name"
	DefaultCluster     = "default_cluster"
	DefaultNameSpace   = "default_namespace"
	DefaultDeployment  = "default_deployment"
	DefaultHostName    = "default_hostname"
	DefaultPodName     = "default_pod"

	DefaultTraceIDHeader = "trace-id"
	CurrentSpanContext   = "current-span-context"
)

var (
	metaOnce   sync.Once
	meta       MetaData
	optionOnce sync.Once
	o          Options

	FailToGetMeta = errors.New("failed to get metadata from context")
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
		o.meta = GetMetaData()
		o.maxBodySize = 10240
		o.serverEnabled = true
		o.clientEnabled = true
		if maxBoxSizeStr := os.Getenv("JM_MAX_BOX_SIZE"); len(maxBoxSizeStr) > 0 {
			maxBodySize, err := strconv.Atoi(maxBoxSizeStr)
			if err == nil {
				o.maxBodySize = maxBodySize
			}
		}
		if serverEnabled := os.Getenv("JM_SERVER_ENABLED"); strings.ToUpper(strings.TrimSpace(serverEnabled)) == "FALSE" {
			o.serverEnabled = false
		}
		if clientEnabled := os.Getenv("JM_CLIENT_ENABLED"); strings.ToUpper(strings.TrimSpace(clientEnabled)) == "FALSE" {
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

func GetMetaData() MetaData {
	metaOnce.Do(func() {
		meta = MetaData{
			ClusterName: DefaultCluster,
			Namespace:   DefaultNameSpace,
			Deployment:  DefaultDeployment,
			HostName:    DefaultHostName,
			PodName:     DefaultPodName,
			ServiceName: DefaultServiceName,
			Environment: DefaultEnvironment,
			TraceHeader: DefaultTraceIDHeader,
			InputHeader: DefaultInputHeader,
		}
		if os.Getenv("JM_CLUSTER_NAME") != "" {
			meta.ClusterName = os.Getenv("JM_CLUSTER_NAME")
		}
		if os.Getenv("JM_NAMESPACE") != "" {
			meta.Namespace = os.Getenv("JM_NAMESPACE")
		}
		if os.Getenv("JM_DEPLOYMENT") != "" {
			meta.Deployment = os.Getenv("JM_DEPLOYMENT")
		}
		if hostname, err := os.Hostname(); err == nil {
			meta.HostName = strings.TrimSpace(hostname)
			meta.PodName = strings.TrimSpace(hostname)
		}
		if os.Getenv("JM_SERVICE_NAME") != "" {
			meta.ServiceName = os.Getenv("JM_SERVICE_NAME")
		}
		if os.Getenv("JM_ENVIRONMENT") != "" {
			meta.Environment = os.Getenv("JM_ENVIRONMENT")
		}
	})
	return meta
}

func Addr(ctx context.Context) (addr string) {
	p, _ := peer.FromContext(ctx)
	if p != nil {
		return p.Addr.String()
	}
	return "unknown peer addr"
}
