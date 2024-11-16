package postgresexporter

import (
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type Config struct {
	TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	QueueSettings             exporterhelper.QueueConfig `mapstructure:"sending_queue"`
	Username                  string                     `mapstructure:"username"`
	Password                  string                     `mapstructure:"password"`
	Database                  string                     `mapstructure:"database"`
	Port                      int                        `mapstructure:"port"`
	Host                      string                     `mapstructure:"host"`
	LogsTableName             string                     `mapstructure:"logs_table_name"`
	TracesTableName           string                     `mapstructure:"traces_table_name"`
	MetricsTableName          string                     `mapstructure:"metrics_table_name"`
}
