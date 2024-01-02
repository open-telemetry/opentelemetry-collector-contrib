// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"

	grpcZap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.uber.org/zap"
)

// CreateLogger creates a logger for use by telemetrygen
func CreateLogger(skipSettingGRPCLogger bool) (*zap.Logger, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain logger: %w", err)
	}
	if !skipSettingGRPCLogger {
		grpcZap.ReplaceGrpcLoggerV2WithVerbosity(logger.WithOptions(
			zap.AddCallerSkip(3),
		), 1) // set to warn verbosity to avoid copious logging from grpc framework
	}
	return logger, err
}
