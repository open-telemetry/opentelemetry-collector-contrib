// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/macosunifiedloggingreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

// unifiedLoggingReceiver uses exec.Command to run the native macOS `log` command
//
//nolint:unused // only used on darwin platform (see config.go)
type unifiedLoggingReceiver struct {
	config   *Config
	logger   *zap.Logger
	consumer consumer.Logs
	cancel   context.CancelFunc
}

//nolint:unused // only used on darwin platform (see config.go)
func newUnifiedLoggingReceiver(
	config *Config,
	logger *zap.Logger,
	consumer consumer.Logs,
) *unifiedLoggingReceiver {
	return &unifiedLoggingReceiver{
		config:   config,
		logger:   logger,
		consumer: consumer,
	}
}

//nolint:unused // only used on darwin platform (see config.go)
func (r *unifiedLoggingReceiver) Start(ctx context.Context, _ component.Host) error {
	r.logger.Info("Starting macOS unified logging receiver")

	_, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	return nil
}

//nolint:unused // only used on darwin platform (see config.go)
func (r *unifiedLoggingReceiver) Shutdown(_ context.Context) error {
	r.logger.Info("Shutting down macOS unified logging receiver")
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}
