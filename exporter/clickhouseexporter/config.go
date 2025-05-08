// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
)

// Config defines configuration for clickhouse exporter.
type Config struct {
	// collectorVersion is the build version of the collector. This is overridden when an exporter is initialized.
	collectorVersion string
	driverName       string // for testing

	TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`

	// Endpoint is the clickhouse endpoint.
	Endpoint string `mapstructure:"endpoint"`
	// Username is the authentication username.
	Username string `mapstructure:"username"`
	// Password is the authentication password.
	Password configopaque.String `mapstructure:"password"`
	// Database is the database name to export.
	Database string `mapstructure:"database"`
	// ConnectionParams is the extra connection parameters with map format. for example compression/dial_timeout
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
	// TableEngine is the table engine to use. default is `MergeTree()`.
	TableEngine TableEngine `mapstructure:"table_engine"`
	// ClusterName if set will append `ON CLUSTER` with the provided name when creating tables.
	ClusterName string `mapstructure:"cluster_name"`
	// CreateSchema if set to true will run the DDL for creating the database and tables. default is true.
	CreateSchema bool `mapstructure:"create_schema"`
	// Compress controls the compression algorithm. Valid options: `none` (disabled), `zstd`, `lz4` (default), `gzip`, `deflate`, `br`, `true` (lz4).
	Compress string `mapstructure:"compress"`
	// AsyncInsert if true will enable async inserts. Default is `true`.
	// Ignored if async inserts are configured in the `endpoint` or `connection_params`.
	// Async inserts may still be overridden server-side.
	AsyncInsert bool `mapstructure:"async_insert"`
	// MetricsTables defines the table names for metric types.
	MetricsTables MetricTablesConfig `mapstructure:"metrics_tables"`
}

type MetricTablesConfig struct {
	// Gauge is the table name for gauge metric type. default is `otel_metrics_gauge`.
	Gauge internal.MetricTypeConfig `mapstructure:"gauge"`
	// Sum is the table name for sum metric type. default is `otel_metrics_sum`.
	Sum internal.MetricTypeConfig `mapstructure:"sum"`
	// Summary is the table name for summary metric type. default is `otel_metrics_summary`.
	Summary internal.MetricTypeConfig `mapstructure:"summary"`
	// Histogram is the table name for histogram metric type. default is `otel_metrics_histogram`.
	Histogram internal.MetricTypeConfig `mapstructure:"histogram"`
	// ExponentialHistogram is the table name for exponential histogram metric type. default is `otel_metrics_exponential_histogram`.
	ExponentialHistogram internal.MetricTypeConfig `mapstructure:"exponential_histogram"`
}

// TableEngine defines the ENGINE string value when creating the table.
type TableEngine struct {
	Name   string `mapstructure:"name"`
	Params string `mapstructure:"params"`
}

const (
	defaultDatabase           = "default"
	defaultTableEngineName    = "MergeTree"
	defaultMetricTableName    = "otel_metrics"
	defaultGaugeSuffix        = "_gauge"
	defaultSumSuffix          = "_sum"
	defaultSummarySuffix      = "_summary"
	defaultHistogramSuffix    = "_histogram"
	defaultExpHistogramSuffix = "_exponential_histogram"
)

var (
	errConfigNoEndpoint      = errors.New("endpoint must be specified")
	errConfigInvalidEndpoint = errors.New("endpoint must be url format")
)

// Validate the ClickHouse server configuration.
func (cfg *Config) Validate() (err error) {
	if cfg.Endpoint == "" {
		err = errors.Join(err, errConfigNoEndpoint)
	}
	dsn, e := cfg.buildDSN()
	if e != nil {
		err = errors.Join(err, e)
	}

	cfg.buildMetricTableNames()

	// Validate DSN with clickhouse driver.
	// Last chance to catch invalid config.
	if _, e := clickhouse.ParseDSN(dsn); e != nil {
		err = errors.Join(err, e)
	}

	return err
}

func (cfg *Config) buildDSN() (string, error) {
	dsnURL, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return "", fmt.Errorf("%w: %s", errConfigInvalidEndpoint, err.Error())
	}

	queryParams := dsnURL.Query()

	// Add connection params to query params.
	for k, v := range cfg.ConnectionParams {
		queryParams.Set(k, v)
	}

	// Enable TLS if scheme is https. This flag is necessary to support https connections.
	if dsnURL.Scheme == "https" {
		queryParams.Set("secure", "true")
	}

	// Use async_insert from config if not specified in DSN.
	if !queryParams.Has("async_insert") {
		queryParams.Set("async_insert", fmt.Sprintf("%t", cfg.AsyncInsert))
	}

	if !queryParams.Has("compress") && (cfg.Compress == "" || cfg.Compress == "true") {
		queryParams.Set("compress", "lz4")
	} else if !queryParams.Has("compress") {
		queryParams.Set("compress", cfg.Compress)
	}

	productInfo := queryParams.Get("client_info_product")
	collectorProductInfo := fmt.Sprintf("%s/%s", "otelcol", cfg.collectorVersion)
	if productInfo == "" {
		productInfo = collectorProductInfo
	} else {
		productInfo = fmt.Sprintf("%s,%s", productInfo, collectorProductInfo)
	}
	queryParams.Set("client_info_product", productInfo)

	// Use database from config if not specified in path, or if config is not default.
	if dsnURL.Path == "" || cfg.Database != defaultDatabase {
		dsnURL.Path = cfg.Database
	}

	// Override username and password if specified in config.
	if cfg.Username != "" {
		dsnURL.User = url.UserPassword(cfg.Username, string(cfg.Password))
	}

	dsnURL.RawQuery = queryParams.Encode()

	return dsnURL.String(), nil
}

func (cfg *Config) buildDB() (*sql.DB, error) {
	dsn, err := cfg.buildDSN()
	if err != nil {
		return nil, err
	}

	// ClickHouse sql driver will read clickhouse settings from the DSN string.
	// It also ensures defaults.
	// See https://github.com/ClickHouse/clickhouse-go/blob/08b27884b899f587eb5c509769cd2bdf74a9e2a1/clickhouse_std.go#L189
	conn, err := sql.Open(cfg.driverName, dsn)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// shouldCreateSchema returns true if the exporter should run the DDL for creating database/tables.
func (cfg *Config) shouldCreateSchema() bool {
	return cfg.CreateSchema
}

func (cfg *Config) buildMetricTableNames() {
	tableName := defaultMetricTableName

	if len(cfg.MetricsTableName) != 0 && !cfg.areMetricTableNamesSet() {
		tableName = cfg.MetricsTableName
	}

	if len(cfg.MetricsTables.Gauge.Name) == 0 {
		cfg.MetricsTables.Gauge.Name = tableName + defaultGaugeSuffix
	}
	if len(cfg.MetricsTables.Sum.Name) == 0 {
		cfg.MetricsTables.Sum.Name = tableName + defaultSumSuffix
	}
	if len(cfg.MetricsTables.Summary.Name) == 0 {
		cfg.MetricsTables.Summary.Name = tableName + defaultSummarySuffix
	}
	if len(cfg.MetricsTables.Histogram.Name) == 0 {
		cfg.MetricsTables.Histogram.Name = tableName + defaultHistogramSuffix
	}
	if len(cfg.MetricsTables.ExponentialHistogram.Name) == 0 {
		cfg.MetricsTables.ExponentialHistogram.Name = tableName + defaultExpHistogramSuffix
	}
}

func (cfg *Config) areMetricTableNamesSet() bool {
	return len(cfg.MetricsTables.Gauge.Name) != 0 ||
		len(cfg.MetricsTables.Sum.Name) != 0 ||
		len(cfg.MetricsTables.Summary.Name) != 0 ||
		len(cfg.MetricsTables.Histogram.Name) != 0 ||
		len(cfg.MetricsTables.ExponentialHistogram.Name) != 0
}

// tableEngineString generates the ENGINE string.
func (cfg *Config) tableEngineString() string {
	engine := cfg.TableEngine.Name
	params := cfg.TableEngine.Params

	if cfg.TableEngine.Name == "" {
		engine = defaultTableEngineName
		params = ""
	}

	return fmt.Sprintf("%s(%s)", engine, params)
}

// clusterString generates the ON CLUSTER string. Returns empty string if not set.
func (cfg *Config) clusterString() string {
	if cfg.ClusterName == "" {
		return ""
	}

	return fmt.Sprintf("ON CLUSTER %s", cfg.ClusterName)
}
