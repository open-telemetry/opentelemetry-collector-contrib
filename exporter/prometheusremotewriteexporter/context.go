package prometheusremotewriteexporter

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

type ctxKey int

const (
	loggerCtxKey ctxKey = iota
)

func contextWithLogger(ctx context.Context, log *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, log)
}

func loggerFromContext(ctx context.Context) (*zap.Logger, error) {
	v := ctx.Value(loggerCtxKey)
	if v == nil {
		return nil, fmt.Errorf("no logger found in context")
	}

	l, ok := v.(*zap.Logger)
	if !ok {
		return nil, fmt.Errorf("invalid logger found in context")
	}

	return l, nil
}
