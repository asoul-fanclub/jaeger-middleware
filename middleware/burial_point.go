package middleware

import "context"

// SelectMethod
// Firstly support Jaeger, and this will be customized by the caller
// TODO: data
type SelectMethod func(ctx context.Context, method string) error
