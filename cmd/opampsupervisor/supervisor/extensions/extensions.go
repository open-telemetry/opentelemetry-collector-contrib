// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/extensions"

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

// Extensions manages the lifecycle of extensions configured in the supervisor.
type Extensions struct {
	extensions map[component.ID]extension.Extension
	// order is the deterministic start/shutdown order of extensions.
	order  []component.ID
	host   *host
	logger *zap.Logger
}

// host is the minimal component.Host implementation exposed to extensions.
type host struct {
	extensions map[component.ID]component.Component
}

func (h *host) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

// New creates and returns a ready-to-start Extensions from already-validated
// configs. Extension instances are created eagerly so that factory errors are
// surfaced before any lifecycle begins. Start must be called to transition the
// extensions to the running state; Shutdown must be called to stop them.
func New(
	ctx context.Context,
	cfg Config,
	factories map[component.Type]extension.Factory,
	telemetry component.TelemetrySettings,
) (*Extensions, error) {
	order := make([]component.ID, 0, len(cfg))
	for id := range cfg {
		order = append(order, id)
	}
	sortIDs(order)

	exts := make(map[component.ID]extension.Extension, len(cfg))
	hostExts := make(map[component.ID]component.Component, len(cfg))
	for _, id := range order {
		factory, ok := factories[id.Type()]
		if !ok {
			return nil, fmt.Errorf("unknown extension type %q for id %q", id.Type(), id)
		}
		ext, err := factory.Create(ctx, extension.Settings{
			ID:                id,
			TelemetrySettings: telemetry,
		}, cfg[id])
		if err != nil {
			return nil, fmt.Errorf("failed to create extension %q: %w", id, err)
		}
		exts[id] = ext
		hostExts[id] = ext
	}

	return &Extensions{
		extensions: exts,
		order:      order,
		host:       &host{extensions: hostExts},
		logger:     telemetry.Logger,
	}, nil
}

// Start starts all extensions in deterministic order. If any extension fails
// to start, already-started extensions are shut down in reverse order.
func (e *Extensions) Start(ctx context.Context) error {
	for i, id := range e.order {
		if err := e.extensions[id].Start(ctx, e.host); err != nil {
			// Roll back already-started extensions in reverse order.
			var rollbackErrs error
			for j := i - 1; j >= 0; j-- {
				rbID := e.order[j]
				if rbErr := e.extensions[rbID].Shutdown(ctx); rbErr != nil {
					rollbackErrs = errors.Join(rollbackErrs, fmt.Errorf("failed to shut down extension %q during rollback: %w", rbID, rbErr))
				}
			}
			if rollbackErrs != nil {
				return errors.Join(fmt.Errorf("failed to start extension %q: %w", id, err), rollbackErrs)
			}
			return fmt.Errorf("failed to start extension %q: %w", id, err)
		}
	}
	return nil
}

// Shutdown stops all extensions in reverse order. Errors from individual
// extensions are joined and returned.
func (e *Extensions) Shutdown(ctx context.Context) error {
	var errs error

	for _, id := range slices.Backward(e.order) {
		if err := e.extensions[id].Shutdown(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to shut down extension %q: %w", id, err))
		}
	}
	return errs
}

// GetExtensions returns the extensions keyed by component ID.
func (e *Extensions) GetExtensions() map[component.ID]component.Component {
	out := make(map[component.ID]component.Component, len(e.extensions))
	for id, ext := range e.extensions {
		out[id] = ext
	}
	return out
}

// sortIDs orders IDs by string form for deterministic startup/shutdown.
// This doesn't provide protection for dependent relationships between extensions.
// Once https://github.com/open-telemetry/opentelemetry-collector/issues/15216 is addressed,
// the majority of this file can be removed and better sorting will be used.
func sortIDs(ids []component.ID) {
	// Simple insertion sort: the list is small (a handful of extensions).
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j-1].String() > ids[j].String(); j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}
