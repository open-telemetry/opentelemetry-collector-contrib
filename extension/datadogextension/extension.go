// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
)

type datadogExtension struct {
	extension.Extension // Embed base Extension for common functionality.
}

var (
	_ extensioncapabilities.ConfigWatcher   = (*datadogExtension)(nil)
	_ extensioncapabilities.PipelineWatcher = (*datadogExtension)(nil)
	_ componentstatus.Watcher               = (*datadogExtension)(nil)
)

// NotifyConfig implements the extensioncapabilities.ConfigWatcher interface, which allows
// this extension to be notified of the Collector's effective configuration.
// This method is called during startup by the Collector's service after calling Start.
func (e *datadogExtension) NotifyConfig(context.Context, *confmap.Conf) error {
	return nil
}

// Ready implements the extensioncapabilities.PipelineWatcher interface.
func (e *datadogExtension) Ready() error {
	return nil
}

// NotReady implements the extensioncapabilities.PipelineWatcher interface.
func (e *datadogExtension) NotReady() error {
	return nil
}

// ComponentStatusChanged implements the componentstatus.Watcher interface.
func (e *datadogExtension) ComponentStatusChanged(
	*componentstatus.InstanceID,
	*componentstatus.Event,
) {
}

// Start starts the extension via the component interface.
func (e *datadogExtension) Start(context.Context, component.Host) error {
	return nil
}

// Shutdown stops the extension via the component interface.
// It shuts down the HTTP server, stops forwarder, and passes signal on
// channel to end goroutine that sends the Datadog otel_collector payloads.
func (e *datadogExtension) Shutdown(context.Context) error {
	return nil
}

func newExtension(
	context.Context,
	*Config,
	extension.Settings,
) (*datadogExtension, error) {
	e := &datadogExtension{}
	return e, nil
}
