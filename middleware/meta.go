package middleware

import (
	"context"
	"os"
	"strings"
	"sync"

	"google.golang.org/grpc/peer"
)

var (
	once sync.Once
	pod  MetaData
)

const (
	Environment          = "ENVIRONMENT"
	ServiceName          = "SERVICE_NAME"
	DefaultEnvironment   = "develop"
	DefaultServiceName   = "trace-demo"
	DefaultTraceIDHeader = "trace-id"
)

type MetaData struct {
	ClusterName string `json:"cluster_name"`
	Namespace   string `json:"namespace"`
	Deployment  string `json:"deployment"`
	PodName     string `json:"pod_name"`
	HostName    string `json:"hostname"`
}

func GetMetaData() MetaData {
	once.Do(func() {
		pod = MetaData{}
		pod.ClusterName = strings.TrimSpace(os.Getenv("CLUSTER_INFO"))
		if len(pod.ClusterName) == 0 {
			pod.ClusterName = "default_cluster"
		}
		pod.Namespace = strings.TrimSpace(os.Getenv("NAMESPACE"))
		pod.Deployment = strings.TrimSpace(os.Getenv("SERVICE_NAME"))
		if len(pod.Namespace) == 0 {
			pod.Namespace = "default_namespace"
			pod.Deployment = "default_service"
		}
		if hostname, err := os.Hostname(); err != nil {
			pod.HostName = "default_hostname"
			pod.PodName = "default_pod"
		} else {
			pod.HostName = strings.TrimSpace(hostname)
			pod.PodName = strings.TrimSpace(hostname)
		}
	})
	return pod
}

func Env() (env string) {
	env = DefaultEnvironment
	if os.Getenv(Environment) != "" {
		env = os.Getenv(Environment)
	}
	return
}

func SName() (serviceName string) {
	serviceName = DefaultServiceName
	if os.Getenv(ServiceName) != "" {
		serviceName = os.Getenv(ServiceName)
	}
	return
}

func Addr(ctx context.Context) (addr string) {
	p, _ := peer.FromContext(ctx)
	if p != nil {
		return p.Addr.String()
	}
	return "unknown peer addr"
}

func TraceHeader() string {
	return DefaultTraceIDHeader
}
