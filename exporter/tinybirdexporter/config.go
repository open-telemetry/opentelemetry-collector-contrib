// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tinybirdexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter"

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

var datasourceRegex = regexp.MustCompile(`^[\w_]+$`)

type SignalConfig struct {
	Datasource string `mapstructure:"datasource"`

	_ struct{}
}

func (cfg SignalConfig) Validate() error {
	if cfg.Datasource == "" {
		return errors.New("datasource cannot be empty")
	}
	if !datasourceRegex.MatchString(cfg.Datasource) {
		return fmt.Errorf("invalid datasource %q: only letters, numbers, and underscores are allowed", cfg.Datasource)
	}
	return nil
}

// Config defines configuration for the Tinybird exporter.
type Config struct {
	ClientConfig confighttp.ClientConfig         `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	RetryConfig  configretry.BackOffConfig       `mapstructure:"retry_on_failure"`
	QueueConfig  exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`

	// Tinybird API token.
	Token   configopaque.String `mapstructure:"token"`
	Metrics metricSignalConfigs `mapstructure:"metrics"`
	Traces  SignalConfig        `mapstructure:"traces"`
	Logs    SignalConfig        `mapstructure:"logs"`
	// Wait for data to be ingested before returning a response.
	Wait bool `mapstructure:"wait"`
}

type metricSignalConfigs struct {
	MetricsGauge                SignalConfig `mapstructure:"gauge"`
	MetricsSum                  SignalConfig `mapstructure:"sum"`
	MetricsHistogram            SignalConfig `mapstructure:"histogram"`
	MetricsExponentialHistogram SignalConfig `mapstructure:"exponential_histogram"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.ClientConfig.Endpoint == "" {
		return errMissingEndpoint
	}
	u, err := url.Parse(cfg.ClientConfig.Endpoint)
	if err != nil {
		return fmt.Errorf("endpoint must be a valid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("endpoint must have http or https scheme: %q", cfg.ClientConfig.Endpoint)
	}
	if u.Host == "" {
		return fmt.Errorf("endpoint must have a host: %q", cfg.ClientConfig.Endpoint)
	}
	if cfg.Token == "" {
		return errMissingToken
	}
	return nil
}
