// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type Config struct {
	// confighttp.ClientConfig.Headers is the headers of doris stream load.
	confighttp.ClientConfig   `mapstructure:",squash"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`

	// TableNames is the table name for logs, traces and metrics.
	Table `mapstructure:"table"`

	// Database is the database name.
	Database string `mapstructure:"database"`
	// Username is the authentication username.
	Username string `mapstructure:"username"`
	// Password is the authentication password.
	Password configopaque.String `mapstructure:"password"`
	// CreateSchema is whether databases and tables are created automatically.
	CreateSchema bool `mapstructure:"create_schema"`
	// MySQLEndpoint is the mysql protocol address to create the schema; ignored if create_schema is false.
	MySQLEndpoint string `mapstructure:"mysql_endpoint"`
	// Data older than these days will be deleted; ignored if create_schema is false. If set to 0, historical data will not be deleted.
	HistoryDays int32 `mapstructure:"history_days"`
	// The number of days in the history partition that was created when the table was created; ignored if create_schema is false.
	// If history_days is not 0, create_history_days needs to be less than or equal to history_days.
	CreateHistoryDays int32 `mapstructure:"create_history_days"`
	// ReplicationNum is the number of replicas of the table; ignored if create_schema is false.
	ReplicationNum int32 `mapstructure:"replication_num"`
	// Timezone is the timezone of the doris.
	TimeZone string `mapstructure:"timezone"`
	// LogResponse is whether to log the response of doris stream load.
	LogResponse bool `mapstructure:"log_response"`
	// LabelPrefix is the prefix of the label in doris stream load.
	LabelPrefix string `mapstructure:"label_prefix"`
	// ProgressInterval is the interval of the progress reporter.
	LogProgressInterval int `mapstructure:"log_progress_interval"`

	// not in config file, will be set in Validate
	timeLocation *time.Location `mapstructure:"-"`
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

		if cfg.CreateHistoryDays < 0 {
			err = errors.Join(err, errors.New("create_history_days must be greater than or equal to 0"))
		}

		if cfg.HistoryDays > 0 && cfg.CreateHistoryDays > cfg.HistoryDays {
			err = errors.Join(err, errors.New("create_history_days must be less than or equal to history_days"))
		}

		if cfg.ReplicationNum < 1 {
			err = errors.Join(err, errors.New("replication_num must be greater than or equal to 1"))
		}
	}

	// Preventing SQL Injection Attacks
	re := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !re.MatchString(cfg.Database) {
		err = errors.Join(err, errors.New("database name must be alphanumeric and underscore"))
	}
	if !re.MatchString(cfg.Logs) {
		err = errors.Join(err, errors.New("logs table name must be alphanumeric and underscore"))
	}
	if !re.MatchString(cfg.Traces) {
		err = errors.Join(err, errors.New("traces table name must be alphanumeric and underscore"))
	}
	if !re.MatchString(cfg.Metrics) {
		err = errors.Join(err, errors.New("metrics table name must be alphanumeric and underscore"))
	}

	var errT error
	cfg.timeLocation, errT = time.LoadLocation(cfg.TimeZone)
	if errT != nil {
		err = errors.Join(err, errors.New("invalid timezone"))
	}

	return err
}

const (
	defaultStart = -2147483648 // IntMin
)

func (cfg *Config) startHistoryDays() int32 {
	if cfg.HistoryDays == 0 {
		return defaultStart
	}
	return -cfg.HistoryDays
}

const (
	properties = `
PROPERTIES (
"replication_num" = "%d",
"compaction_policy" = "time_series",
"dynamic_partition.enable" = "true",
"dynamic_partition.create_history_partition" = "true",
"dynamic_partition.time_unit" = "DAY",
"dynamic_partition.start" = "%d",
"dynamic_partition.history_partition_num" = "%d",
"dynamic_partition.end" = "1",
"dynamic_partition.prefix" = "p"
)
`
)

func (cfg *Config) propertiesStr() string {
	return fmt.Sprintf(properties, cfg.ReplicationNum, cfg.startHistoryDays(), cfg.CreateHistoryDays)
}
