// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package starrocksexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver for StarRocks
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal/metrics"
)

// Config defines configuration for starrocks exporter.
type Config struct {
	// collectorVersion is the build version of the collector. This is overridden when an exporter is initialized.
	collectorVersion string

	TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	QueueSettings             configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`

	// Endpoint is the StarRocks MySQL endpoint (e.g., "localhost:9030").
	Endpoint string `mapstructure:"endpoint"`
	// Username is the authentication username.
	Username string `mapstructure:"username"`
	// Password is the authentication password. Empty password is supported.
	Password configopaque.String `mapstructure:"password"`
	// Database is the database name to export.
	Database string `mapstructure:"database"`
	// TLS is the TLS config for connecting to StarRocks.
	TLS configtls.ClientConfig `mapstructure:"tls"`
	// ConnectionParams is the extra connection parameters with map format.
	ConnectionParams map[string]string `mapstructure:"connection_params"`
	// LogsTableName is the table name for logs. default is `otel_logs`.
	LogsTableName string `mapstructure:"logs_table_name"`
	// TracesTableName is the table name for traces. default is `otel_traces`.
	TracesTableName string `mapstructure:"traces_table_name"`
	// MetricsTableName is the table name for metrics. default is `otel_metrics`.
	//
	// Deprecated: MetricsTableName exists for historical compatibility
	// and should not be used. To set the metrics tables name,
	// use the MetricsTables parameter instead.
	MetricsTableName string `mapstructure:"metrics_table_name"`
	// TTL is The data time-to-live example 30m, 48h. 0 means no ttl.
	TTL time.Duration `mapstructure:"ttl"`
	// CreateSchema if set to true will run the DDL for creating the database and tables. default is true.
	CreateSchema bool `mapstructure:"create_schema"`
	// MetricsTables defines the table names for metric types.
	MetricsTables MetricTablesConfig `mapstructure:"metrics_tables"`
	// MaxOpenConns is the maximum number of open connections to the database.
	MaxOpenConns int `mapstructure:"max_open_conns"`
	// MaxIdleConns is the maximum number of idle connections in the pool.
	MaxIdleConns int `mapstructure:"max_idle_conns"`
	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	// ConnMaxIdleTime is the maximum amount of time a connection may be idle before being closed.
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

type MetricTablesConfig struct {
	// Gauge is the table name for gauge metric type. default is `otel_metrics_gauge`.
	Gauge metrics.MetricTypeConfig `mapstructure:"gauge"`
	// Sum is the table name for sum metric type. default is `otel_metrics_sum`.
	Sum metrics.MetricTypeConfig `mapstructure:"sum"`
	// Summary is the table name for summary metric type. default is `otel_metrics_summary`.
	Summary metrics.MetricTypeConfig `mapstructure:"summary"`
	// Histogram is the table name for histogram metric type. default is `otel_metrics_histogram`.
	Histogram metrics.MetricTypeConfig `mapstructure:"histogram"`
	// ExponentialHistogram is the table name for exponential histogram metric type. default is `otel_metrics_exponential_histogram`.
	ExponentialHistogram metrics.MetricTypeConfig `mapstructure:"exponential_histogram"`
}

const (
	defaultDatabase           = "default"
	defaultMetricTableName    = "otel_metrics"
	defaultGaugeSuffix        = "_gauge"
	defaultSumSuffix          = "_sum"
	defaultSummarySuffix      = "_summary"
	defaultHistogramSuffix    = "_histogram"
	defaultExpHistogramSuffix = "_exponential_histogram"
	defaultMaxOpenConns       = 3
	defaultMaxIdleConns       = 1
	defaultConnMaxLifetime    = 5 * time.Minute
	defaultConnMaxIdleTime    = 30 * time.Second
)

var (
	errConfigNoEndpoint      = errors.New("endpoint must be specified")
	errConfigInvalidEndpoint = errors.New("endpoint must be in format host:port")
)

func createDefaultConfig() component.Config {
	return &Config{
		collectorVersion: "unknown",

		TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
		QueueSettings:    configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		BackOffConfig:    configretry.NewDefaultBackOffConfig(),
		ConnectionParams: map[string]string{},
		Database:         defaultDatabase,
		LogsTableName:    "otel_logs",
		TracesTableName:  "otel_traces",
		TTL:              0,
		CreateSchema:     true,
		MaxOpenConns:     defaultMaxOpenConns,
		MaxIdleConns:     defaultMaxIdleConns,
		ConnMaxLifetime:  defaultConnMaxLifetime,
		ConnMaxIdleTime:  defaultConnMaxIdleTime,
		MetricsTables: MetricTablesConfig{
			Gauge:                metrics.MetricTypeConfig{Name: defaultMetricTableName + defaultGaugeSuffix},
			Sum:                  metrics.MetricTypeConfig{Name: defaultMetricTableName + defaultSumSuffix},
			Summary:              metrics.MetricTypeConfig{Name: defaultMetricTableName + defaultSummarySuffix},
			Histogram:            metrics.MetricTypeConfig{Name: defaultMetricTableName + defaultHistogramSuffix},
			ExponentialHistogram: metrics.MetricTypeConfig{Name: defaultMetricTableName + defaultExpHistogramSuffix},
		},
	}
}

// Validate the StarRocks server configuration.
func (cfg *Config) Validate() (err error) {
	if cfg.Endpoint == "" {
		err = errors.Join(err, errConfigNoEndpoint)
	}

	dsn, e := cfg.buildDSN()
	if e != nil {
		err = errors.Join(err, e)
	}

	cfg.buildMetricTableNames()

	// Validate DSN by attempting to parse it
	if dsn != "" {
		_, e := sql.Open("mysql", dsn)
		if e != nil {
			err = errors.Join(err, fmt.Errorf("invalid DSN: %w", e))
		}
	}

	return err
}

func (cfg *Config) buildDSN() (string, error) {
	// StarRocks uses MySQL protocol, so we build a MySQL DSN
	// Format: username:password@tcp(host:port)/database?params
	// MySQL driver handles special characters in password without URL encoding
	password := string(cfg.Password)
	// Empty password is supported: format will be username:@tcp(...)

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s",
		cfg.Username,
		password,
		cfg.Endpoint,
		cfg.Database)

	// Add connection parameters
	if len(cfg.ConnectionParams) > 0 {
		params := url.Values{}
		for k, v := range cfg.ConnectionParams {
			params.Set(k, v)
		}
		dsn += "?" + params.Encode()
	}

	return dsn, nil
}

func (cfg *Config) buildStarRocksDB() (*sql.DB, error) {
	dsn, err := cfg.buildDSN()
	if err != nil {
		return nil, fmt.Errorf("failed to build DSN from config: %w", err)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// shouldCreateSchema returns true if the exporter should run the DDL for creating database/tables.
func (cfg *Config) shouldCreateSchema() bool {
	return cfg.CreateSchema
}

func (cfg *Config) buildMetricTableNames() {
	tableName := defaultMetricTableName

	if cfg.MetricsTableName != "" && !cfg.areMetricTableNamesSet() {
		tableName = cfg.MetricsTableName
	}

	if cfg.MetricsTables.Gauge.Name == "" {
		cfg.MetricsTables.Gauge.Name = tableName + defaultGaugeSuffix
	}
	if cfg.MetricsTables.Sum.Name == "" {
		cfg.MetricsTables.Sum.Name = tableName + defaultSumSuffix
	}
	if cfg.MetricsTables.Summary.Name == "" {
		cfg.MetricsTables.Summary.Name = tableName + defaultSummarySuffix
	}
	if cfg.MetricsTables.Histogram.Name == "" {
		cfg.MetricsTables.Histogram.Name = tableName + defaultHistogramSuffix
	}
	if cfg.MetricsTables.ExponentialHistogram.Name == "" {
		cfg.MetricsTables.ExponentialHistogram.Name = tableName + defaultExpHistogramSuffix
	}
}

func (cfg *Config) areMetricTableNamesSet() bool {
	return cfg.MetricsTables.Gauge.Name != "" ||
		cfg.MetricsTables.Sum.Name != "" ||
		cfg.MetricsTables.Summary.Name != "" ||
		cfg.MetricsTables.Histogram.Name != "" ||
		cfg.MetricsTables.ExponentialHistogram.Name != ""
}

// database returns the preferred database for creating tables and inserting data.
func (cfg *Config) database() string {
	if cfg.Database != "" && cfg.Database != defaultDatabase {
		return cfg.Database
	}
	return defaultDatabase
}
