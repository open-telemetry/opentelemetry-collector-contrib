// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombmarkerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
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

	confighttp.ClientConfig   `mapstructure:",squash"`
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

type Marker struct {
	// Type defines the type of Marker.
	Type string `mapstructure:"type"`

	// MessageKey is the attribute that will be used as the message.
	// If necessary the value will be converted to a string.
	MessageKey string `mapstructure:"message_key"`

	// URLKey is the attribute that will be used as the url.
	// If necessary the value will be converted to a string.
	URLKey string `mapstructure:"url_key"`

	// Rules are the OTTL rules that determine when a piece of telemetry should be turned into a Marker
	Rules Rules `mapstructure:"rules"`

	// DatasetSlug is the endpoint that specifies the Honeycomb environment
	DatasetSlug string `mapstructure:"dataset_slug"`
}

type Rules struct {
	// LogConditions is the list of ottllog conditions that determine a match
	LogConditions []string `mapstructure:"log_conditions"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	if cfg.APIKey == "" {
		return errors.New("invalid API Key")
	}

	if len(cfg.Markers) == 0 {
		return errors.New("no markers supplied")
	}
	for _, m := range cfg.Markers {
		if m.Type == "" {
			return fmt.Errorf("marker must have a type %v", m)
		}

		if len(m.Rules.LogConditions) == 0 {
			return fmt.Errorf("marker must have rules %v", m)
		}

		_, err := filterottl.NewBoolExprForLog(m.Rules.LogConditions, filterottl.StandardLogFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()})
		if err != nil {
			return err
		}
	}

	return nil
}

var _ component.Config = (*Config)(nil)
