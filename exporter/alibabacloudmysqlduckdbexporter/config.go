// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudmysqlduckdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for the MySQL exporter.
type Config struct {
	TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	QueueSettings             configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	// Endpoint is the MySQL DSN.
	// Format: user:password@tcp(host:port)/dbname?params
	// Example for Alibaba Cloud RDS: root:pass@tcp(rm-xxx.mysql.rds.aliyuncs.com:3306)/otel?tls=true&parseTime=true
	Endpoint string `mapstructure:"endpoint"`
	// Database is the database name.
	Database string `mapstructure:"database"`
	// LogsTableName is the table name for logs. Default is "otel_logs".
	LogsTableName string `mapstructure:"logs_table_name"`
	// TracesTableName is the table name for traces. Default is "otel_traces".
	TracesTableName string `mapstructure:"traces_table_name"`
	// MetricsTableName is the base table name prefix for metrics. Default is "otel_metrics".
	MetricsTableName string `mapstructure:"metrics_table_name"`
	// CreateSchema if set to true will run the DDL for creating the database and tables. Default is true.
	CreateSchema bool `mapstructure:"create_schema"`
}

const (
	defaultDatabase         = "otel"
	defaultLogsTableName    = "otel_logs"
	defaultTracesTableName  = "otel_traces"
	defaultMetricsTableName = "otel_metrics"
)

var errConfigNoEndpoint = errors.New("endpoint must be specified")

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
		BackOffConfig:   configretry.NewDefaultBackOffConfig(),
		QueueSettings:   configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		Database:        defaultDatabase,
		LogsTableName:   defaultLogsTableName,
		TracesTableName: defaultTracesTableName,
		MetricsTableName: defaultMetricsTableName,
		CreateSchema:    true,
	}
}

// Validate the MySQL configuration.
func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return errConfigNoEndpoint
	}
	return nil
}

// buildDSN constructs the DSN string with parseTime=true appended if not present.
func (cfg *Config) buildDSN() string {
	dsn := cfg.Endpoint
	// go-sql-driver/mysql requires parseTime=true to scan DATETIME into time.Time
	if len(dsn) > 0 {
		hasQuery := false
		for _, c := range dsn {
			if c == '?' {
				hasQuery = true
				break
			}
		}
		if hasQuery {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
	}
	return dsn
}

func (cfg *Config) gaugeTableName() string {
	return cfg.MetricsTableName + "_gauge"
}

func (cfg *Config) sumTableName() string {
	return cfg.MetricsTableName + "_sum"
}

func (cfg *Config) histogramTableName() string {
	return cfg.MetricsTableName + "_histogram"
}

func (cfg *Config) summaryTableName() string {
	return cfg.MetricsTableName + "_summary"
}

func renderCreateTableSQL(template, database, tableName string) string {
	return fmt.Sprintf(template, database, tableName)
}

