// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter"

import (
	"context"
	"errors"

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
		return nil, errors.New("no logger found in context")
	}

	l, ok := v.(*zap.Logger)
	if !ok {
		return nil, errors.New("invalid logger found in context")
	}

	return l, nil
}
