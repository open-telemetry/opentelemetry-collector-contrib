// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

var _ extension.Extension = (*ecsObserver)(nil)

// ecsObserver implements component.ServiceExtension interface.
type ecsObserver struct {
	logger *zap.Logger
	sd     *serviceDiscovery

	// for Shutdown
	cancel func()
}

// Start runs the service discovery in background
func (e *ecsObserver) Start(_ context.Context, host component.Host) error {
	e.logger.Info("Starting ECSDiscovery")
	// Ignore the ctx parameter as it is not for long running operation
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	go func() {
		if err := e.sd.runAndWriteFile(ctx); err != nil {
			e.logger.Error("ECSDiscovery stopped by error", zap.Error(err))
			// Stop the collector
			host.ReportFatalError(err)
		}
	}()
	return nil
}

func (e *ecsObserver) Shutdown(ctx context.Context) error {
	e.logger.Info("Stopping ECSDiscovery")
	e.cancel()
	return nil
}
