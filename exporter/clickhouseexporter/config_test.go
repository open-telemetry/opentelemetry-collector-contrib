// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/metadata"
)

const defaultEndpoint = "clickhouse://127.0.0.1:9000"

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultCfg := createDefaultConfig()
	defaultCfg.(*Config).Endpoint = defaultEndpoint

	storageID := component.MustNewIDWithName("file_storage", "clickhouse")

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: defaultCfg,
		},
		{
			id: component.NewIDWithName(metadata.Type, "full"),
			expected: &Config{
				collectorVersion: "unknown",
				driverName:       clickhouseDriverName,
				Endpoint:         defaultEndpoint,
				Database:         "otel",
				Username:         "foo",
				Password:         "bar",
				TTL:              72 * time.Hour,
				LogsTableName:    "otel_logs",
				TracesTableName:  "otel_traces",
				CreateSchema:     true,
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 5 * time.Second,
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     5 * time.Second,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      300 * time.Second,
					RandomizationFactor: backoff.DefaultRandomizationFactor,
					Multiplier:          backoff.DefaultMultiplier,
				},
				MetricsTables: MetricTablesConfig{
					Gauge:                internal.MetricTypeConfig{Name: "otel_metrics_custom_gauge"},
					Sum:                  internal.MetricTypeConfig{Name: "otel_metrics_custom_sum"},
					Summary:              internal.MetricTypeConfig{Name: "otel_metrics_custom_summary"},
					Histogram:            internal.MetricTypeConfig{Name: "otel_metrics_custom_histogram"},
					ExponentialHistogram: internal.MetricTypeConfig{Name: "otel_metrics_custom_exp_histogram"},
				},
				ConnectionParams: map[string]string{},
				QueueSettings: exporterhelper.QueueBatchConfig{
					Enabled:      true,
					NumConsumers: 10,
					QueueSize:    100,
					StorageID:    &storageID,
					Sizer:        exporterhelper.RequestSizerTypeRequests,
				},
				AsyncInsert: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := createDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}

func TestBuildMetricMetricTableNames(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want Config
	}{
		{
			name: "nothing set",
			cfg:  Config{},
			want: Config{
				MetricsTables: MetricTablesConfig{
					Gauge:                internal.MetricTypeConfig{Name: "otel_metrics_gauge"},
					Sum:                  internal.MetricTypeConfig{Name: "otel_metrics_sum"},
					Summary:              internal.MetricTypeConfig{Name: "otel_metrics_summary"},
					Histogram:            internal.MetricTypeConfig{Name: "otel_metrics_histogram"},
					ExponentialHistogram: internal.MetricTypeConfig{Name: "otel_metrics_exponential_histogram"},
				},
			},
		},
		{
			name: "only metric_table_name set",
			cfg: Config{
				MetricsTableName: "table_name",
			},
			want: Config{
				MetricsTableName: "table_name",
				MetricsTables: MetricTablesConfig{
					Gauge:                internal.MetricTypeConfig{Name: "table_name_gauge"},
					Sum:                  internal.MetricTypeConfig{Name: "table_name_sum"},
					Summary:              internal.MetricTypeConfig{Name: "table_name_summary"},
					Histogram:            internal.MetricTypeConfig{Name: "table_name_histogram"},
					ExponentialHistogram: internal.MetricTypeConfig{Name: "table_name_exponential_histogram"},
				},
			},
		},
		{
			name: "only metric_tables set fully",
			cfg: Config{
				MetricsTables: MetricTablesConfig{
					Gauge:                internal.MetricTypeConfig{Name: "table_name_gauge"},
					Sum:                  internal.MetricTypeConfig{Name: "table_name_sum"},
					Summary:              internal.MetricTypeConfig{Name: "table_name_summary"},
					Histogram:            internal.MetricTypeConfig{Name: "table_name_histogram"},
					ExponentialHistogram: internal.MetricTypeConfig{Name: "table_name_exponential_histogram"},
				},
			},
			want: Config{
				MetricsTables: MetricTablesConfig{
					Gauge:                internal.MetricTypeConfig{Name: "table_name_gauge"},
					Sum:                  internal.MetricTypeConfig{Name: "table_name_sum"},
					Summary:              internal.MetricTypeConfig{Name: "table_name_summary"},
					Histogram:            internal.MetricTypeConfig{Name: "table_name_histogram"},
					ExponentialHistogram: internal.MetricTypeConfig{Name: "table_name_exponential_histogram"},
				},
			},
		},
		{
			name: "only metric_tables set partially",
			cfg: Config{
				MetricsTables: MetricTablesConfig{
					Summary:              internal.MetricTypeConfig{Name: "table_name_summary"},
					Histogram:            internal.MetricTypeConfig{Name: "table_name_histogram"},
					ExponentialHistogram: internal.MetricTypeConfig{Name: "table_name_exp_histogram"},
				},
			},
			want: Config{
				MetricsTables: MetricTablesConfig{
					Gauge:                internal.MetricTypeConfig{Name: "otel_metrics_gauge"},
					Sum:                  internal.MetricTypeConfig{Name: "otel_metrics_sum"},
					Summary:              internal.MetricTypeConfig{Name: "table_name_summary"},
					Histogram:            internal.MetricTypeConfig{Name: "table_name_histogram"},
					ExponentialHistogram: internal.MetricTypeConfig{Name: "table_name_exp_histogram"},
				},
			},
		},
		{
			name: "only metric_tables set partially with metric_table_name",
			cfg: Config{
				MetricsTableName: "custom_name",
				MetricsTables: MetricTablesConfig{
					Summary:              internal.MetricTypeConfig{Name: "table_name_summary"},
					Histogram:            internal.MetricTypeConfig{Name: "table_name_histogram"},
					ExponentialHistogram: internal.MetricTypeConfig{Name: "table_name_exp_histogram"},
				},
			},
			want: Config{
				MetricsTableName: "custom_name",
				MetricsTables: MetricTablesConfig{
					Gauge:                internal.MetricTypeConfig{Name: "otel_metrics_gauge"},
					Sum:                  internal.MetricTypeConfig{Name: "otel_metrics_sum"},
					Summary:              internal.MetricTypeConfig{Name: "table_name_summary"},
					Histogram:            internal.MetricTypeConfig{Name: "table_name_histogram"},
					ExponentialHistogram: internal.MetricTypeConfig{Name: "table_name_exp_histogram"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.buildMetricTableNames()
			require.Equal(t, tt.want, tt.cfg)
		})
	}
}

func TestAreMetricTableNamesSet(t *testing.T) {
	cfg := Config{}
	require.False(t, cfg.areMetricTableNamesSet())

	cfg = Config{
		MetricsTables: MetricTablesConfig{
			Gauge: internal.MetricTypeConfig{Name: "gauge"},
		},
	}
	require.True(t, cfg.areMetricTableNamesSet())
}

func TestConfig_buildDSN(t *testing.T) {
	type fields struct {
		Endpoint         string
		Username         string
		Password         string
		Database         string
		Compress         string
		ConnectionParams map[string]string
		AsyncInsert      *bool
	}
	mergeConfigWithFields := func(cfg *Config, fields fields) {
		if fields.Endpoint != "" {
			cfg.Endpoint = fields.Endpoint
		}
		if fields.Username != "" {
			cfg.Username = fields.Username
		}
		if fields.Password != "" {
			cfg.Password = configopaque.String(fields.Password)
		}
		if fields.Database != "" {
			cfg.Database = fields.Database
		}
		if fields.ConnectionParams != nil {
			cfg.ConnectionParams = fields.ConnectionParams
		}
		if fields.Compress != "" {
			cfg.Compress = fields.Compress
		}
		if fields.AsyncInsert != nil {
			cfg.AsyncInsert = *fields.AsyncInsert
		}
	}

	type ChOptions struct {
		Secure      bool
		DialTimeout time.Duration
		Compress    clickhouse.CompressionMethod
	}

	configTrue := true
	configFalse := false
	tests := []struct {
		name          string
		fields        fields
		want          string
		wantChOptions ChOptions
		wantErr       error
	}{
		{
			name: "valid default config",
			fields: fields{
				Endpoint: defaultEndpoint,
			},
			wantChOptions: ChOptions{
				Secure: false,
			},
			want: "clickhouse://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "Support tcp scheme",
			fields: fields{
				Endpoint: "tcp://127.0.0.1:9000",
			},
			wantChOptions: ChOptions{
				Secure: false,
			},
			want: "tcp://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "prefers database name from config over from DSN",
			fields: fields{
				Endpoint: defaultEndpoint,
				Username: "foo",
				Password: "bar",
				Database: "otel",
			},
			wantChOptions: ChOptions{
				Secure: false,
			},
			want: "clickhouse://foo:bar@127.0.0.1:9000/otel?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "use database name from DSN if not set in config",
			fields: fields{
				Endpoint: "clickhouse://foo:bar@127.0.0.1:9000/otel",
				Username: "foo",
				Password: "bar",
			},
			wantChOptions: ChOptions{
				Secure: false,
			},
			want: "clickhouse://foo:bar@127.0.0.1:9000/otel?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "invalid config",
			fields: fields{
				Endpoint: "127.0.0.1:9000",
			},
			wantChOptions: ChOptions{
				Secure: false,
			},
			wantErr: errConfigInvalidEndpoint,
		},
		{
			name: "Auto enable TLS connection based on scheme",
			fields: fields{
				Endpoint: "https://127.0.0.1:9000",
			},
			wantChOptions: ChOptions{
				Secure: true,
			},
			want: "https://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4&secure=true",
		},
		{
			name: "Preserve query parameters",
			fields: fields{
				Endpoint: "clickhouse://127.0.0.1:9000?secure=true&compress=lz4&foo=bar",
			},
			wantChOptions: ChOptions{
				Secure: true,
			},
			want: "clickhouse://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4&foo=bar&secure=true",
		},
		{
			name: "Parse clickhouse settings",
			fields: fields{
				Endpoint: "https://127.0.0.1:9000?secure=true&dial_timeout=30s&compress=br",
			},
			wantChOptions: ChOptions{
				Secure:      true,
				DialTimeout: 30 * time.Second,
				Compress:    clickhouse.CompressionBrotli,
			},
			want: "https://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=br&dial_timeout=30s&secure=true",
		},
		{
			name: "Should respect connection parameters",
			fields: fields{
				Endpoint:         "clickhouse://127.0.0.1:9000?foo=bar",
				ConnectionParams: map[string]string{"secure": "true"},
			},
			wantChOptions: ChOptions{
				Secure: true,
			},
			want: "clickhouse://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4&foo=bar&secure=true",
		},
		{
			name: "support replace database in DSN with config to override database",
			fields: fields{
				Endpoint: "tcp://127.0.0.1:9000/otel",
				Database: "override",
			},
			want: "tcp://127.0.0.1:9000/override?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "when config option is missing, preserve async_insert false in DSN",
			fields: fields{
				Endpoint: "tcp://127.0.0.1:9000?async_insert=false",
			},
			want: "tcp://127.0.0.1:9000/default?async_insert=false&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "when config option is missing, preserve async_insert true in DSN",
			fields: fields{
				Endpoint: "tcp://127.0.0.1:9000?async_insert=true",
			},
			want: "tcp://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "ignore config option when async_insert is present in connection params as false",
			fields: fields{
				Endpoint:         "tcp://127.0.0.1:9000?async_insert=false",
				ConnectionParams: map[string]string{"async_insert": "false"},
				AsyncInsert:      &configTrue,
			},

			want: "tcp://127.0.0.1:9000/default?async_insert=false&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "ignore config option when async_insert is present in connection params as true",
			fields: fields{
				Endpoint:         "tcp://127.0.0.1:9000?async_insert=false",
				ConnectionParams: map[string]string{"async_insert": "true"},
				AsyncInsert:      &configFalse,
			},

			want: "tcp://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "ignore config option when async_insert is present in DSN as false",
			fields: fields{
				Endpoint:    "tcp://127.0.0.1:9000?async_insert=false",
				AsyncInsert: &configTrue,
			},

			want: "tcp://127.0.0.1:9000/default?async_insert=false&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "use async_insert true config option when it is not present in DSN",
			fields: fields{
				Endpoint:    "tcp://127.0.0.1:9000",
				AsyncInsert: &configTrue,
			},

			want: "tcp://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "use async_insert false config option when it is not present in DSN",
			fields: fields{
				Endpoint:    "tcp://127.0.0.1:9000",
				AsyncInsert: &configFalse,
			},

			want: "tcp://127.0.0.1:9000/default?async_insert=false&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "set async_insert to true when not present in config or DSN",
			fields: fields{
				Endpoint: "tcp://127.0.0.1:9000",
			},

			want: "tcp://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "connection_params takes priority over endpoint and async_insert option.",
			fields: fields{
				Endpoint:         "tcp://127.0.0.1:9000?async_insert=false",
				ConnectionParams: map[string]string{"async_insert": "true"},
				AsyncInsert:      &configFalse,
			},

			want: "tcp://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "use compress br config option when it is not present in DSN",
			fields: fields{
				Endpoint: "tcp://127.0.0.1:9000",
				Compress: "br",
			},

			want: "tcp://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=br",
		},
		{
			name: "set compress to lz4 when not present in config or DSN",
			fields: fields{
				Endpoint: "tcp://127.0.0.1:9000",
			},

			want: "tcp://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "connection_params takes priority over endpoint and compress option.",
			fields: fields{
				Endpoint:         "tcp://127.0.0.1:9000?compress=none",
				ConnectionParams: map[string]string{"compress": "br"},
				Compress:         "lz4",
			},
			want: "tcp://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=br",
		},
		{
			name: "include default otel product info in DSN",
			fields: fields{
				Endpoint: "tcp://127.0.0.1:9000",
			},

			want: "tcp://127.0.0.1:9000/default?async_insert=true&client_info_product=otelcol%2Ftest&compress=lz4",
		},
		{
			name: "correctly append default product info when value is included in DSN",
			fields: fields{
				Endpoint: "tcp://127.0.0.1:9000?client_info_product=customProductInfo%2Fv1.2.3",
			},

			want: "tcp://127.0.0.1:9000/default?async_insert=true&client_info_product=customProductInfo%2Fv1.2.3%2Cotelcol%2Ftest&compress=lz4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.collectorVersion = "test"
			mergeConfigWithFields(cfg, tt.fields)
			dsn, err := cfg.buildDSN()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr, "buildDSN()")
			} else {
				// Validate DSN
				opts, err := clickhouse.ParseDSN(dsn)
				assert.NoError(t, err)
				assert.Equalf(t, tt.wantChOptions.Secure, opts.TLS != nil, "TLSConfig is not nil")
				assert.Equalf(t, tt.wantChOptions.DialTimeout, opts.DialTimeout, "DialTimeout is not nil")
				if tt.wantChOptions.Compress != 0 {
					assert.Equalf(t, tt.wantChOptions.Compress, opts.Compression.Method, "Compress is not nil")
				}
				assert.Equalf(t, tt.want, dsn, "buildDSN()")
			}
		})
	}
}

func TestShouldCreateSchema(t *testing.T) {
	t.Parallel()

	caseDefault := createDefaultConfig().(*Config)
	caseCreateSchemaTrue := createDefaultConfig().(*Config)
	caseCreateSchemaTrue.CreateSchema = true
	caseCreateSchemaFalse := createDefaultConfig().(*Config)
	caseCreateSchemaFalse.CreateSchema = false

	tests := []struct {
		name     string
		input    *Config
		expected bool
	}{
		{
			name:     "default",
			input:    caseDefault,
			expected: true,
		},
		{
			name:     "true",
			input:    caseCreateSchemaTrue,
			expected: true,
		},
		{
			name:     "false",
			input:    caseCreateSchemaFalse,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("ShouldCreateSchema case %s", tt.name), func(t *testing.T) {
			assert.NoError(t, xconfmap.Validate(tt))
			assert.Equal(t, tt.expected, tt.input.shouldCreateSchema())
		})
	}
}

func TestTableEngineConfigParsing(t *testing.T) {
	t.Parallel()
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected string
	}{
		{
			id:       component.NewIDWithName(metadata.Type, "table-engine-empty"),
			expected: "MergeTree()",
		},
		{
			id:       component.NewIDWithName(metadata.Type, "table-engine-name-only"),
			expected: "ReplicatedReplacingMergeTree()",
		},
		{
			id:       component.NewIDWithName(metadata.Type, "table-engine-full"),
			expected: "ReplicatedReplacingMergeTree('/clickhouse/tables/{shard}/table_name', '{replica}', ver)",
		},
		{
			id:       component.NewIDWithName(metadata.Type, "table-engine-params-only"),
			expected: "MergeTree()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg.(*Config).tableEngineString())
		})
	}
}

func TestClusterString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "",
			expected: "",
		},
		{
			input:    "cluster_a_b",
			expected: "ON CLUSTER cluster_a_b",
		},
		{
			input:    "cluster a b",
			expected: "ON CLUSTER cluster a b",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("ClusterString case %d", i), func(t *testing.T) {
			cfg := createDefaultConfig()
			cfg.(*Config).Endpoint = defaultEndpoint
			cfg.(*Config).ClusterName = tt.input

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg.(*Config).clusterString())
		})
	}
}
