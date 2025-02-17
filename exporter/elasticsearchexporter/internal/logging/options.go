// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package logging contains utility functions for logging.
package logging

import (
	"math"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// WithRateLimit returns a zap.Option which rate limits messages
// with approximately the given frequency.
func WithRateLimit(interval time.Duration) zap.Option {
	return zap.WrapCore(func(in zapcore.Core) zapcore.Core {
		return zapcore.NewSamplerWithOptions(in, interval, 1, math.MaxInt32)
	})
}
