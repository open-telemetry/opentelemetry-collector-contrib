// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"database/sql"
	"errors"
	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"
)

// Config defines configuration for Elastic exporter.
type Config struct {
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
	// QueueSettings is a subset of exporterhelper.QueueSettings,
	// because only QueueSize is user-settable.
	QueueSettings QueueSettings `mapstructure:"sending_queue"`

	// Endpoint is the clickhouse endpoint.
	Endpoint string `mapstructure:"endpoint"`
	// Database is the database name to export.
	Database string `mapstructure:"database"`
	// Username is the authentication username.
	Username string `mapstructure:"username"`
	// Username is the authentication password.
	Password string `mapstructure:"password"`
	// LogsTableName is the table name for logs. default is `otel_logs`.
	LogsTableName string `mapstructure:"logs_table_name"`
	// TracesTableName is the table name for logs. default is `otel_traces`.
	TracesTableName string `mapstructure:"traces_table_name"`
	// MetricsTableName is the table name for metrics. default is `otel_metrics`.
	MetricsTableName string `mapstructure:"metrics_table_name"`
	// TTLDays is The data time-to-live in days, 0 means no ttl.
	TTLDays uint `mapstructure:"ttl_days"`
}

// QueueSettings is a subset of exporterhelper.QueueSettings.
type QueueSettings struct {
	// QueueSize set the length of the sending queue
	QueueSize int `mapstructure:"queue_size"`
}

const defaultDatabase = "default"

var (
	errConfigNoHost     = errors.New("host must be specified")
	errConfigInvalidDSN = errors.New("DSN is invalid")
)

// Validate the clickhouse server configuration.
func (cfg *Config) Validate() (err error) {
	if cfg.Endpoint == "" {
		err = multierr.Append(err, errConfigNoHost)
	}
	_, e := cfg.buildDB(cfg.Database)
	if e != nil {
		err = multierr.Append(err, e)
	}
	return err
}

func (cfg *Config) enforcedQueueSettings() exporterhelper.QueueSettings {
	return exporterhelper.QueueSettings{
		Enabled:      true,
		NumConsumers: 1,
		QueueSize:    cfg.QueueSettings.QueueSize,
	}
}

func (cfg *Config) buildDBOptions(database string) (*clickhouse.Options, error) {
	opts, err := clickhouse.ParseDSN(cfg.Endpoint)
	if err != nil {
		return nil, errConfigInvalidDSN
	}

	// Override database if specified.
	// If not specified, use the database from the DSN.
	if database != "" {
		opts.Auth.Database = database
	}

	// Override auth if specified.
	// If not specified, use the auth from the DSN.
	if cfg.Username != "" {
		opts.Auth.Username = cfg.Username
		opts.Auth.Password = cfg.Password
	}

	return opts, nil
}

func (cfg *Config) buildDB(database string) (*sql.DB, error) {
	opts, err := cfg.buildDBOptions(database)
	if err != nil {
		return nil, err
	}

	// Before opening the db, OpenDB ensure connection defaults (if not overridden).
	conn := clickhouse.OpenDB(opts)

	return conn, nil

}
