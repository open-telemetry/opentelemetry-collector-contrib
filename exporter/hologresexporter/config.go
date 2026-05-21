// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/hologresexporter"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Hologres exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`
	configretry.BackOffConfig    `mapstructure:"retry_on_failure"`
	QueueSettings                configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	// DSN is the PostgreSQL connection string for Hologres.
	// Format: postgresql://user:password@host:port/database?sslmode=disable
	DSN string `mapstructure:"dsn"`

	// TracesTableName is the name of the table for traces data.
	TracesTableName string `mapstructure:"traces_table_name"`

	// LogsTableName is the name of the table for logs data.
	LogsTableName string `mapstructure:"logs_table_name"`

	// MetricsTableName is the prefix for metrics tables.
	// Actual tables will be: {prefix}_gauge, {prefix}_sum, {prefix}_histogram,
	// {prefix}_summary, {prefix}_exp_histogram
	MetricsTableName string `mapstructure:"metrics_table_name"`

	// CreateSchema indicates whether to create tables on startup.
	CreateSchema bool `mapstructure:"create_schema"`

	// TTL is the time-to-live for data. Used to set partition_expiration_time.
	// Set to 0 to disable TTL.
	TTL time.Duration `mapstructure:"ttl"`
}

// Validate checks if the configuration is valid.
func (cfg *Config) Validate() error {
	if cfg.DSN == "" {
		return errors.New("dsn must be specified")
	}
	if cfg.TracesTableName == "" {
		return errors.New("traces_table_name must not be empty")
	}
	if cfg.LogsTableName == "" {
		return errors.New("logs_table_name must not be empty")
	}
	if cfg.MetricsTableName == "" {
		return errors.New("metrics_table_name must not be empty")
	}
	return nil
}
