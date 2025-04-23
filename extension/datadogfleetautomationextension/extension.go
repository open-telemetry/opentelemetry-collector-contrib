// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogfleetautomationextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogfleetautomationextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"
)

type fleetAutomationExtension struct {
	extension.Extension // Embed base Extension for common functionality.
}

var (
	_ extensioncapabilities.ConfigWatcher   = (*fleetAutomationExtension)(nil)
	_ extensioncapabilities.PipelineWatcher = (*fleetAutomationExtension)(nil)
	_ componentstatus.Watcher               = (*fleetAutomationExtension)(nil)
)

// NotifyConfig implements the extensioncapabilities.ConfigWatcher interface, which allows
// this extension to be notified of the Collector's effective configuration.
// This method is called during startup by the Collector's service after calling Start.
func (e *fleetAutomationExtension) NotifyConfig(context.Context, *confmap.Conf) error {
	return nil
}

// Ready implements the extensioncapabilities.PipelineWatcher interface.
func (e *fleetAutomationExtension) Ready() error {
	return nil
}

// NotReady implements the extensioncapabilities.PipelineWatcher interface.
func (e *fleetAutomationExtension) NotReady() error {
	return nil
}

// ComponentStatusChanged implements the componentstatus.Watcher interface.
func (e *fleetAutomationExtension) ComponentStatusChanged(
	*componentstatus.InstanceID,
	*componentstatus.Event,
) {
}

// Start starts the extension via the component interface.
func (e *fleetAutomationExtension) Start(context.Context, component.Host) error {
	return nil
}

// Shutdown stops the extension via the component interface.
// It shuts down the HTTP server, stops forwarder, and passes signal on
// channel to end goroutine that sends the Datadog fleet automation payloads.
func (e *fleetAutomationExtension) Shutdown(context.Context) error {
	return nil
}

func newExtension(
	context.Context,
	*Config,
	extension.Settings,
) (*fleetAutomationExtension, error) {
	e := &fleetAutomationExtension{}
	return e, nil
}
