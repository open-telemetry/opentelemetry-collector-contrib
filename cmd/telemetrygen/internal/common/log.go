// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zapgrpc"
	"google.golang.org/grpc/grpclog"
)

// CreateLogger creates a logger for use by telemetrygen
func CreateLogger(skipSettingGRPCLogger bool) (*zap.Logger, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain logger: %w", err)
	}

	if !skipSettingGRPCLogger {
		grpcLogger := zapgrpc.NewLogger(logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			return core.With([]zapcore.Field{zap.Bool("grpc_log", true)})
		}), zap.AddCallerSkip(3)))
		grpclog.SetLoggerV2(grpcLogger)
	}
	return logger, err
}
