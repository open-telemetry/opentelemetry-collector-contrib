// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type Config struct {
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	configretry.BackOffConfig      `mapstructure:"retry_on_failure"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`

	// TableNames is the table name for logs, traces and metrics.
	Table `mapstructure:"table"`

	// Endpoint is the http stream load address.
	Endpoint string `mapstructure:"endpoint"`
	// Database is the database name.
	Database string `mapstructure:"database"`
	// Username is the authentication username.
	Username string `mapstructure:"username"`
	// Password is the authentication password.
	Password string `mapstructure:"password"`
	// CreateSchema is whether databases and tables are created automatically.
	CreateSchema bool `mapstructure:"create_schema"`
	// MySQLEndpoint is the mysql protocol address to create the schema; ignored if create_schema is false.
	MySQLEndpoint string `mapstructure:"mysql_endpoint"`
	// Data older than these days will be deleted; ignored if create_schema is false. If set to 0, historical data will not be deleted.
	HistoryDays int32 `mapstructure:"history_days"`
	// Timezone is the timezone of the doris.
	TimeZone string `mapstructure:"timezone"`
}

type Table struct {
	// Logs is the table name for logs.
	Logs string `mapstructure:"logs"`
	// Traces is the table name for traces.
	Traces string `mapstructure:"traces"`
	// Metrics is the table name for metrics.
	Metrics string `mapstructure:"metrics"`
}

func (cfg *Config) Validate() (err error) {
	if cfg.Endpoint == "" {
		err = errors.Join(err, errors.New("endpoint must be specified"))
	}
	if cfg.CreateSchema {
		if cfg.MySQLEndpoint == "" {
			err = errors.Join(err, errors.New("mysql_endpoint must be specified"))
		}

		if cfg.HistoryDays < 0 {
			err = errors.Join(err, errors.New("history_days must be greater than or equal to 0"))
		}
	}

	return err
}

const (
	defaultStart       = -2147483648 // IntMin
	defaultHistoryDays = 498
)

func (cfg *Config) startAndHistoryDays() (int32, int32) {
	if cfg.HistoryDays == 0 {
		return defaultStart, defaultHistoryDays
	}
	if cfg.HistoryDays > 498 {
		return -cfg.HistoryDays, defaultHistoryDays
	}
	return -cfg.HistoryDays, cfg.HistoryDays
}

func (cfg *Config) timeZone() (*time.Location, error) {
	return time.LoadLocation(cfg.TimeZone)
}
