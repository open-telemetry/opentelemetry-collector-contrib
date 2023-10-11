// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombmarkerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// Config defines configuration for the Honeycomb Marker exporter.
type Config struct {
	// APIKey is the authentication token associated with the Honeycomb account.
	APIKey configopaque.String `mapstructure:"api_key"`

	// API URL to use (defaults to https://api.honeycomb.io)
	APIURL string `mapstructure:"api_url"`

	// Markers is the list of markers to create
	Markers []Marker `mapstructure:"markers"`
}

type Marker struct {
	// Type defines the type of Marker.  Markers with the same type appear in Honeycomb with the same color
	Type string `mapstructure:"type"`

	// Color is the color of the Marker. Will only be used if the Type does not already exist.
	Color string `mapstructure:"color"`

	// MessageField is the attribute that will be used as the message.
	// If necessary the value will be converted to a string.
	MessageField string `mapstructure:"message_field"`

	// URLField is the attribute that will be used as the url.
	// If necessary the value will be converted to a string.
	URLField string `mapstructure:"url_field"`

	// Rules are the OTTL rules that determine when a piece of telemetry should be turned into a Marker
	Rules Rules `mapstructure:"rules"`
}

type Rules struct {
	// ResourceConditions is the list of ottlresource conditions that determine a match
	ResourceConditions []string `mapstructure:"resource_conditions"`

	// LogConditions is the list of ottllog conditions that determine a match
	LogConditions []string `mapstructure:"log_conditions"`
}

var defaultCfg = createDefaultConfig().(*Config)

func (cfg *Config) Validate() error {
	if cfg == nil {
		cfg = defaultCfg
	}

	if cfg.APIKey == "" {
		return fmt.Errorf("invalid API Key")
	}

	if len(cfg.Markers) != 0 {
		for _, m := range cfg.Markers {
			if len(m.Rules.ResourceConditions) == 0 && len(m.Rules.LogConditions) == 0 {
				return fmt.Errorf("no rules supplied for Marker %v", m)
			}

			_, err := filterottl.NewBoolExprForResource(m.Rules.ResourceConditions, filterottl.StandardResourceFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()})
			if err != nil {
				return err
			}

			_, err = filterottl.NewBoolExprForLog(m.Rules.LogConditions, filterottl.StandardLogFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()})
			if err != nil {
				return err
			}
		}
	} else {
		return fmt.Errorf("no markers supplied")
	}

	return nil
}

var _ component.Config = (*Config)(nil)
