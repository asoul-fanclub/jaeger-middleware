package middleware

import "context"

type SelectMethod func(ctx context.Context, method string) error
