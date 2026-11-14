package observability

import "context"

type operationKey struct{}

func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, operationKey{}, operation)
}

func Operation(ctx context.Context) string {
	operation, _ := ctx.Value(operationKey{}).(string)
	return operation
}
