// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/extensions"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

// Config is the set of supervisor extension configurations keyed by component ID.
// It implements confmap.Unmarshaler so each entry is unmarshaled into the
// concrete config type produced by its factory's CreateDefaultConfig.
type Config map[component.ID]component.Config

// Unmarshal turns the raw extensions block into typed configs using the
// supervisor's allowlisted factories. Validation is left to a top-level
// confmap.Validate call so error paths are reported with full context.
func (c *Config) Unmarshal(conf *confmap.Conf) error {
	factories := Factories()

	// Unmarshaling into a map keyed by component.ID triggers
	// component.ID.UnmarshalText for each YAML key, handling both "type" and
	// "type/name" forms.
	rawCfgs := make(map[component.ID]map[string]any)
	if err := conf.Unmarshal(&rawCfgs); err != nil {
		return fmt.Errorf("failed to parse extensions config: %w", err)
	}

	*c = make(Config, len(rawCfgs))
	for id := range rawCfgs {
		factory, ok := factories[id.Type()]
		if !ok {
			return fmt.Errorf("unknown extension type %q for id %q", id.Type(), id)
		}

		sub, err := conf.Sub(id.String())
		if err != nil {
			return fmt.Errorf("failed to read config for extension %q: %w", id, err)
		}

		cfg := factory.CreateDefaultConfig()
		if err := sub.Unmarshal(&cfg); err != nil {
			return fmt.Errorf("failed to unmarshal config for extension %q: %w", id, err)
		}

		(*c)[id] = cfg
	}

	return nil
}
