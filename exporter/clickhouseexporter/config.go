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
	"errors"
	"net/url"

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

	// Endpoint is the clickhouse server endpoint.
	// TCP endpoint: tcp://ip1:port,ip2:port
	// HTTP endpoint: http://ip:port,ip2:port
	Endpoint string `mapstructure:"endpoint"`
	// Username is the authentication username.
	Username string `mapstructure:"username"`
	// Username is the authentication password.
	Password string `mapstructure:"password"`
	// Database is the database name to export.
	Database string `mapstructure:"database"`
	// ConnectionParams is the extra connection parameters with map format. for example compression/dial_timeout
	ConnectionParams map[string]string `mapstructure:"connection_params"`
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

var (
	errConfigNoEndpoint      = errors.New("endpoint must be specified")
	errConfigInvalidEndpoint = errors.New("endpoint must be url format")
)

// Validate validates the clickhouse server configuration.
func (cfg *Config) Validate() (err error) {
	if cfg.Endpoint == "" {
		err = multierr.Append(err, errConfigNoEndpoint)
	}
	_, e := cfg.buildDSN(cfg.Database)
	if e != nil {
		err = multierr.Append(err, e)
	}
	return err
}

const defaultDatabase = "default"

func (cfg *Config) enforcedQueueSettings() exporterhelper.QueueSettings {
	return exporterhelper.QueueSettings{
		Enabled:      true,
		NumConsumers: 1,
		QueueSize:    cfg.QueueSettings.QueueSize,
	}
}

func (cfg *Config) buildDSN(database string) (string, error) {
	dsn, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return "", errConfigInvalidEndpoint
	}
	if cfg.Username != "" {
		dsn.User = url.UserPassword(cfg.Username, cfg.Password)
	}
	dsn.Path = "/" + database
	params := url.Values{}
	for k, v := range cfg.ConnectionParams {
		params.Set(k, v)
	}
	dsn.RawQuery = params.Encode()
	return dsn.String(), nil
}
