package middleware

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	name     = "default_service_name"
	inputKey = "input_param"
)

// Options
// TODO: header config
type Options struct {
	tracer      trace.Tracer
	maxBodySize int
}

func DefaultOptions() Options {
	o := Options{}
	o.tracer = otel.Tracer(name)
	o.maxBodySize = 10240
	return o
}
