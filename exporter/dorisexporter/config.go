// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type Config struct {
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	configretry.BackOffConfig      `mapstructure:"retry_on_failure"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`

	// Endpoint is the http stream load address and mysql protocol tcp address.
	Endpoint `mapstructure:"endpoint"`
	// TableNames is the table name for logs, traces and metrics.
	Table `mapstructure:"table"`

	// Database is the database name.
	Database string `mapstructure:"database"`
	// Username is the authentication username.
	Username string `mapstructure:"username"`
	// Password is the authentication password.
	Password string `mapstructure:"password"`
	// CreateSchema is whether databases and tables are created automatically.
	CreateSchema bool `mapstructure:"create_schema"`
	// HistoryDays is the number of historical partitions of days to be created; ignored if create_schema is false. Maximum is 500.
	HistoryDays int `mapstructure:"history_days"`
}

type Endpoint struct {
	// HTTP is the http stream load address.
	HTTP string `mapstructure:"http"`
	// TCP is the mysql protocol tcp address; ignored if create_schema is false.
	TCP string `mapstructure:"tcp"`
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
	if cfg.Endpoint.HTTP == "" {
		err = errors.Join(err, errors.New("endpoint.http must be specified"))
	}
	if cfg.CreateSchema {
		if cfg.Endpoint.TCP == "" {
			err = errors.Join(err, errors.New("endpoint.tcp must be specified"))
		}

		if cfg.HistoryDays < 0 || cfg.HistoryDays > 500 {
			err = errors.Join(err, errors.New("history_days must be between 0 and 500"))
		}
	}

	return err
}
