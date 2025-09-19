// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package healthcheckextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensioncapabilities"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck"
)

// compatibilityWrapper wraps the shared healthcheck extension to provide
// backward compatibility for Ready/NotReady calls by translating them
// to component status events that the shared implementation understands.
type compatibilityWrapper struct {
	*healthcheck.HealthCheckExtension
	// Internal component ID used for status tracking
	internalComponentID *componentstatus.InstanceID
}

var (
	_ component.Component                   = (*compatibilityWrapper)(nil)
	_ extension.Extension                   = (*compatibilityWrapper)(nil)
	_ extensioncapabilities.PipelineWatcher = (*compatibilityWrapper)(nil)
	_ extensioncapabilities.ConfigWatcher   = (*compatibilityWrapper)(nil)
)

func newCompatibilityWrapper(sharedExt *healthcheck.HealthCheckExtension) *compatibilityWrapper {
	// Create an internal component ID for tracking status
	internalID := componentstatus.NewInstanceID(
		component.NewID(component.MustNewType("internal_healthcheck")),
		component.KindExtension,
	)

	return &compatibilityWrapper{
		HealthCheckExtension: sharedExt,
		internalComponentID:  internalID,
	}
}

// Start implements the component.Component interface.
func (w *compatibilityWrapper) Start(ctx context.Context, host component.Host) error {
	err := w.HealthCheckExtension.Start(ctx, host)
	if err != nil {
		return err
	}

	// Initialize with starting status to match original behavior
	w.ComponentStatusChanged(
		w.internalComponentID,
		componentstatus.NewEvent(componentstatus.StatusStarting),
	)

	return nil
}

// Ready implements the extensioncapabilities.PipelineWatcher interface.
// This provides backward compatibility by translating Ready calls to StatusOK events.
func (w *compatibilityWrapper) Ready() error {
	// First call the original Ready method for pipeline lifecycle
	err := w.HealthCheckExtension.Ready()
	if err != nil {
		return err
	}

	// Then trigger a component status change to make the health endpoint report OK
	// This bridges the old Ready() behavior to the new event-driven system
	w.ComponentStatusChanged(
		w.internalComponentID,
		componentstatus.NewEvent(componentstatus.StatusOK),
	)

	return nil
}

// NotReady implements the extensioncapabilities.PipelineWatcher interface.
// This provides backward compatibility by translating NotReady calls to unavailable status.
func (w *compatibilityWrapper) NotReady() error {
	// First call the original NotReady method for pipeline lifecycle
	err := w.HealthCheckExtension.NotReady()
	if err != nil {
		return err
	}

	// Then trigger a component status change to make the health endpoint report unavailable
	// This bridges the old NotReady() behavior to the new event-driven system
	w.ComponentStatusChanged(
		w.internalComponentID,
		componentstatus.NewEvent(componentstatus.StatusStarting), // Starting status = unavailable
	)

	return nil
}

// Shutdown implements the component.Component interface.
func (w *compatibilityWrapper) Shutdown(ctx context.Context) error {
	// Don't send status events during shutdown to avoid deadlocks
	// The shared implementation will handle proper cleanup
	return w.HealthCheckExtension.Shutdown(ctx)
}

// NotifyConfig implements the extensioncapabilities.ConfigWatcher interface.
func (w *compatibilityWrapper) NotifyConfig(ctx context.Context, conf *confmap.Conf) error {
	return w.HealthCheckExtension.NotifyConfig(ctx, conf)
}
