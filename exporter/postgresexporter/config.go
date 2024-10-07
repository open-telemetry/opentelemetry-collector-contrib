package postgresexporter

import (
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type Config struct {
	QueueSettings    exporterhelper.QueueConfig `mapstructure:"sending_queue"`
	Username         string                     `mapstructure:"username"`
	Password         string                     `mapstructure:"password"`
	Database         string                     `mapstructure:"database"`
	Port             string                     `mapstructure:"port"`
	Host             string                     `mapstructure:"host"`
	LogsTableName    string                     `mapstructure:"logs_table_name"`
	TracesTableName  string                     `mapstructure:"traces_table_name"`
	MetricsTableName string                     `mapstructure:"metrics_table_name"`
}
