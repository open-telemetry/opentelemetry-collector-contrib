// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckv2extension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

type healthCheckExtension struct {
	config    Config
	telemetry component.TelemetrySettings
}

var _ component.Component = (*healthCheckExtension)(nil)

func newExtension(
	_ context.Context,
	config Config,
	set extension.Settings,
) *healthCheckExtension {
	return &healthCheckExtension{
		config:    config,
		telemetry: set.TelemetrySettings,
	}
}

// Start implements the component.Component interface.
func (hc *healthCheckExtension) Start(context.Context, component.Host) error {
	hc.telemetry.Logger.Debug("Starting health check extension V2", zap.Any("config", hc.config))

	return nil
}

// Shutdown implements the component.Component interface.
func (hc *healthCheckExtension) Shutdown(context.Context) error {
	return nil
}
