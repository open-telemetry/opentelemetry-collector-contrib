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
	telemetrySettings component.TelemetrySettings
	sd                *serviceDiscovery

	// for Shutdown
	cancel func()
}

// Start runs the service discovery in background
func (e *ecsObserver) Start(_ context.Context, _ component.Host) error {
	e.telemetrySettings.Logger.Info("Starting ECSDiscovery")
	// Ignore the ctx parameter as it is not for long running operation
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	go func() {
		if err := e.sd.runAndWriteFile(ctx); err != nil {
			e.telemetrySettings.Logger.Error("ECSDiscovery stopped by error", zap.Error(err))
			// Stop the collector
			_ = e.telemetrySettings.ReportComponentStatus(component.NewFatalErrorEvent(err))
		}
	}()
	return nil
}

func (e *ecsObserver) Shutdown(_ context.Context) error {
	e.telemetrySettings.Logger.Info("Stopping ECSDiscovery")
	e.cancel()
	return nil
}
