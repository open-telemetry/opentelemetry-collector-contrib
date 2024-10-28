// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sematextexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sematextexporter"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type Config struct {
	confighttp.ClientConfig   `mapstructure:",squash"`
	QueueSettings             exporterhelper.QueueConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`

	// App token specifies the token of Sematext Monitoring App to which the user wants to send metrics to.
	AppToken string `mapstructure:"app_token"`

	// Region specifies the Sematext region the user is operating in
	// Options:
	// - EU
	// - US
	Region string `mapstructure:"region"`

	// MetricsSchema indicates the metrics schema to emit to line protocol.
	// Options:
	// - telegraf-prometheus-v2
	MetricsSchema string `mapstructure:"metrics_schema"`

	// PayloadMaxLines is the maximum number of line protocol lines to POST in a single request.
	PayloadMaxLines int `mapstructure:"payload_max_lines"`
	// PayloadMaxBytes is the maximum number of line protocol bytes to POST in a single request.
	PayloadMaxBytes int `mapstructure:"payload_max_bytes"`
}

// Validate checks for invalid or missing entries in the configuration.
func (cfg *Config) Validate() error {
	if cfg.MetricsSchema != "telegraf-prometheus-v2" {
		return fmt.Errorf("invalid metrics schema: %s", cfg.MetricsSchema)
	}
	if strings.ToLower(cfg.Region) != "eu" && strings.ToLower(cfg.Region) != "us" && strings.ToLower(cfg.Region) != "custom" {
		return fmt.Errorf("invalid region: %s. please use either 'EU' or 'US'", cfg.Region)
	}
	if len(cfg.AppToken) != 36 {
		return fmt.Errorf("invalid app_token: %s. app_token should be 36 characters", cfg.AppToken)
	}
	if strings.ToLower(cfg.Region) == "eu" {
		cfg.Endpoint = "https://spm-receiver.eu.sematext.com"
	}
	if strings.ToLower(cfg.Region) == "us" {
		cfg.Endpoint = "https://spm-receiver.sematext.com"
	}

	return nil
}
