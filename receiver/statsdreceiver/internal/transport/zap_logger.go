// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"

import "go.uber.org/zap"

// ZapLogger provides a Logger that emits debug logs on a zap sugared logger
type ZapLogger struct {
	logger        *zap.Logger
	sugaredLogger *zap.SugaredLogger
}

var _ Logger = (*ZapLogger)(nil)

// NewZapLogger returns a new instance of a ZapLogger given an existing zap Logger instance
func NewZapLogger(logger *zap.Logger) Logger {
	return &ZapLogger{
		logger:        logger,
		sugaredLogger: logger.Sugar(),
	}
}

func (zl *ZapLogger) OnDebugf(template string, args ...any) {
	if zl.logger.Check(zap.DebugLevel, "debug") != nil {
		zl.sugaredLogger.Debugf(template, args...)
	}
}
