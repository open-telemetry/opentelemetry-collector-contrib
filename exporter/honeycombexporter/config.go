// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter"

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/zap"
)

// Config defines configuration for the Honeycomb Marker exporter.
type Config struct {
	// APIKey is the authentication token associated with the Honeycomb account.
	APIKey configopaque.String `mapstructure:"api_key"`

	// API URL to use (defaults to https://api.honeycomb.io)
	APIURL string `mapstructure:"api_url"`

	// Markers is the list of markers to create
	Markers []marker `mapstructure:"markers"`

	Logs filterprocessor.LogFilters `mapstructure:"logs"`

	Resources filterprocessor.ResourceFilters `mapstructure:"logs"`
}

type marker struct {
	// MarkerType defines the type of marker.  Markers with the same type appear in Honeycomb with the same color
	MarkerType string `mapstructure:"type"`

	// MarkerColor is the color of the marker. Will only be used if the MarkerType does not already exist.
	MarkerColor string `mapstructure:"color"`

	// MessageField is the attribute that will be used as the message.
	// If necessary the value will be converted to a string.
	MessageField string `mapstructure:"message_field"`

	// UrlField is the attribute that will be used as the url.
	// If necessary the value will be converted to a string.
	UrlField string `mapstructure:"url_field"`

	// Rules are the OTTL rules that determine when a piece of telemetry should be turned into a Marker
	Rules Rules `mapstructure:"rules"`
}

type Rules struct {
	// ResourceConditions is the list of ottlresource conditions that determine a match
	ResourceConditions []string `mapstructure:"resource_conditions"`

	// LogConditions is the list of ottllog conditions that determine a match
	LogConditions []string `mapstructure:"log_conditions"`
}

func (cfg *Config) Validate() error {
	if cfg.APIKey == "" {
		return fmt.Errorf("invalid API Key")
	}

	if cfg.APIURL == "" {
		return fmt.Errorf("invalid URL")
	}

	if len(cfg.Markers) != 0 {
		for _, m := range cfg.Markers {
			if len(m.Rules.ResourceConditions) == 0 && len(m.Rules.LogConditions) == 0 {
				return fmt.Errorf("no rules supplied for marker %v", m)
			}

		}
	} else {
		return fmt.Errorf("no markers supplied")
	}

	if cfg.Logs.LogConditions != nil {
		_, err := filterottl.NewBoolExprForLog(cfg.Logs.LogConditions, filterottl.StandardLogFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()})
		return err
	}

	if cfg.Logs.LogConditions != nil && cfg.Logs.Include != nil {
		return cfg.Logs.Include.Validate()
	}

	if cfg.Logs.LogConditions != nil && cfg.Logs.Exclude != nil {
		return cfg.Logs.Exclude.Validate()
	}

	if cfg.Resources.ResourceConditions != nil {
		_, err := filterottl.NewBoolExprForLog(cfg.Resources.ResourceConditions, filterottl.StandardLogFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()})
		return err
	}

	return nil
}

var _ component.Config = (*Config)(nil)
