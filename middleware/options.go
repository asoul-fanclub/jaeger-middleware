package middleware

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	name = "default_service_name"
)

type Options struct {
	tracer trace.Tracer
}

func DefaultOptions() Options {
	o := Options{}
	o.tracer = otel.Tracer(name)
	return o
}
