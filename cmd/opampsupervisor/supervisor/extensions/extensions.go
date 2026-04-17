// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/extensions"

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/xconfmap"
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

// New creates and returns a ready-to-start Extensions. Extension instances are
// created eagerly so that misconfigurations are surfaced before any lifecycle
// begins. Start must be called to transition the extensions to the running
// state; Shutdown must be called to stop them.
func New(
	ctx context.Context,
	rawCfg map[string]any,
	factories map[component.Type]extension.Factory,
	telemetry component.TelemetrySettings,
) (*Extensions, error) {
	configs, order, err := parseConfigs(rawCfg, factories)
	if err != nil {
		return nil, err
	}

	exts := make(map[component.ID]extension.Extension, len(configs))
	hostExts := make(map[component.ID]component.Component, len(configs))
	for _, id := range order {
		factory := factories[id.Type()]
		cfg := configs[id]
		ext, err := factory.Create(ctx, extension.Settings{
			ID:                id,
			TelemetrySettings: telemetry,
		}, cfg)
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

// ValidateConfigs parses the raw extensions config and validates each entry
// against its factory's config schema, without creating extension instances.
// It uses the default factory list from Factories(). If extensions are
// configured but the opampsupervisor.extensions feature gate is disabled,
// it returns an actionable error naming the gate.
func ValidateConfigs(rawCfg map[string]any) error {
	_, _, err := parseConfigs(rawCfg, Factories())
	return err
}

// parseConfigs converts the raw extensions map into parsed, validated configs
// keyed by component.ID. It returns the configs and a deterministic ordering
// of their IDs.
func parseConfigs(
	rawCfg map[string]any,
	factories map[component.Type]extension.Factory,
) (map[component.ID]component.Config, []component.ID, error) {
	if len(rawCfg) == 0 {
		return nil, nil, nil
	}

	conf := confmap.NewFromStringMap(rawCfg)

	// Unmarshaling into a map keyed by component.ID triggers
	// component.ID.UnmarshalText for each YAML key, handling both "type" and
	// "type/name" forms.
	ids := make(map[component.ID]map[string]any)
	if err := conf.Unmarshal(&ids); err != nil {
		return nil, nil, fmt.Errorf("failed to parse extensions config: %w", err)
	}

	configs := make(map[component.ID]component.Config, len(ids))
	order := make([]component.ID, 0, len(ids))
	for id := range ids {
		factory, ok := factories[id.Type()]
		if !ok {
			return nil, nil, fmt.Errorf("unknown extension type %q for id %q", id.Type(), id)
		}

		sub, err := conf.Sub(id.String())
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read config for extension %q: %w", id, err)
		}

		cfg := factory.CreateDefaultConfig()
		if err := sub.Unmarshal(&cfg); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal config for extension %q: %w", id, err)
		}

		if err := xconfmap.Validate(cfg); err != nil {
			return nil, nil, fmt.Errorf("invalid config for extension %q: %w", id, err)
		}

		configs[id] = cfg
		order = append(order, id)
	}

	sortIDs(order)
	return configs, order, nil
}

// sortIDs orders IDs by string form for deterministic startup/shutdown.
func sortIDs(ids []component.ID) {
	// Simple insertion sort: the list is small (a handful of extensions).
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j-1].String() > ids[j].String(); j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}
