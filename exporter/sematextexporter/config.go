// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sematextexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sematextexporter"

import (
	"fmt"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	euRegion          = "eu"
	usRegion          = "us"
	euMetricsEndpoint = "https://spm-receiver.eu.sematext.com"
	usMetricsEndpoint = "https://spm-receiver.sematext.com"
	usLogsEndpoint    = "https://logsene-receiver.sematext.com"
	euLogsEndpoint    = "https://logsene-receiver.eu.sematext.com"
)

type Config struct {
	confighttp.ClientConfig   `mapstructure:",squash"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	// Region specifies the Sematext region the user is operating in
	// Options:
	// - EU
	// - US
	Region string `mapstructure:"region"`
	// MetricsConfig defines the configuration specific to metrics
	MetricsConfig `mapstructure:"metrics"`
	// LogsConfig defines the configuration specific to logs
	LogsConfig `mapstructure:"logs"`
}
type MetricsConfig struct {
	// App token is the token of Sematext Monitoring App to which you want to send the metrics.
	AppToken string `mapstructure:"app_token"`
	// MetricsEndpoint specifies the endpoint for receiving metrics in Sematext
	MetricsEndpoint string `mapstructure:"-"`
	// MetricsSchema indicates the metrics schema to emit to line protocol.
	// Default: telegraf-prometheus-v2
	MetricsSchema string `mapstructure:"-"`
	// PayloadMaxLines is the maximum number of line protocol lines to POST in a single request.
	PayloadMaxLines int `mapstructure:"payload_max_lines"`
	// PayloadMaxBytes is the maximum number of line protocol bytes to POST in a single request.
	PayloadMaxBytes int `mapstructure:"payload_max_bytes"`
}
type LogsConfig struct {
	// App token is the token of Sematext Monitoring App to which you want to send the logs.
	AppToken string `mapstructure:"app_token"`
	// LogsEndpoint specifies the endpoint for receiving logs in Sematext
	LogsEndpoint string `mapstructure:"-"`
}

// Validate checks for invalid or missing entries in the configuration.
func (cfg *Config) Validate() error {
	if strings.ToLower(cfg.Region) != euRegion && strings.ToLower(cfg.Region) != usRegion && strings.ToLower(cfg.Region) != "" {
		return fmt.Errorf("invalid region: %s. please use either 'EU' or 'US'", cfg.Region)
	}
	if !isValidUUID(cfg.MetricsConfig.AppToken) && cfg.MetricsConfig.AppToken != "" {
		return fmt.Errorf("invalid metrics app_token: %s. app_token is not a valid UUID", cfg.MetricsConfig.AppToken)
	}
	if !isValidUUID(cfg.LogsConfig.AppToken) && cfg.LogsConfig.AppToken != "" {
		return fmt.Errorf("invalid logs app_token: %s. app_token is not a valid UUID", cfg.LogsConfig.AppToken)
	}

	if strings.ToLower(cfg.Region) == euRegion {
		cfg.MetricsEndpoint = euMetricsEndpoint
		cfg.LogsEndpoint = euLogsEndpoint
	}
	if strings.ToLower(cfg.Region) == usRegion {
		cfg.MetricsEndpoint = usMetricsEndpoint
		cfg.LogsEndpoint = usLogsEndpoint
	}

	return nil
}

func isValidUUID(uuid string) bool {
	const uuidPattern = `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	return regexp.MustCompile(uuidPattern).MatchString(strings.ToLower(uuid))
}
