// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
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
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/metadata"
)

const defaultEndpoint = "clickhouse://127.0.0.1:9000"

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultCfg := createDefaultConfig()
	defaultCfg.(*Config).Endpoint = defaultEndpoint

	storageID := component.NewIDWithName(component.Type("file_storage"), "clickhouse")

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
				Endpoint:         defaultEndpoint,
				Database:         "otel",
				Username:         "foo",
				Password:         "bar",
				TTL:              72 * time.Hour,
				LogsTableName:    "otel_logs",
				TracesTableName:  "otel_traces",
				MetricsTableName: "otel_metrics",
				TimeoutSettings: exporterhelper.TimeoutSettings{
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
				ConnectionParams: map[string]string{},
				QueueSettings: exporterhelper.QueueSettings{
					Enabled:      true,
					NumConsumers: 1,
					QueueSize:    100,
					StorageID:    &storageID,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
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

func TestConfig_buildDSN(t *testing.T) {
	type fields struct {
		Endpoint         string
		Username         string
		Password         string
		Database         string
		ConnectionParams map[string]string
	}
	type args struct {
		database string
	}
	type ChOptions struct {
		Secure      bool
		DialTimeout time.Duration
		Compress    clickhouse.CompressionMethod
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		want          string
		wantChOptions ChOptions
		wantErr       error
	}{
		{
			name: "valid default config",
			fields: fields{
				Endpoint: defaultEndpoint,
			},
			args: args{},
			wantChOptions: ChOptions{
				Secure: false,
			},
			want: "clickhouse://127.0.0.1:9000/default",
		},
		{
			name: "Support tcp scheme",
			fields: fields{
				Endpoint: "tcp://127.0.0.1:9000",
			},
			args: args{},
			wantChOptions: ChOptions{
				Secure: false,
			},
			want: "tcp://127.0.0.1:9000/default",
		},
		{
			name: "prefers database name from config over from DSN",
			fields: fields{
				Endpoint: defaultEndpoint,
				Username: "foo",
				Password: "bar",
				Database: "otel",
			},
			args: args{
				database: "otel",
			},
			wantChOptions: ChOptions{
				Secure: false,
			},
			want: "clickhouse://foo:bar@127.0.0.1:9000/otel",
		},
		{
			name: "use database name from DSN if not set in config",
			fields: fields{
				Endpoint: "clickhouse://foo:bar@127.0.0.1:9000/otel",
				Username: "foo",
				Password: "bar",
				Database: "",
			},
			args: args{
				database: "",
			},
			wantChOptions: ChOptions{
				Secure: false,
			},
			want: "clickhouse://foo:bar@127.0.0.1:9000/otel",
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
			args: args{},
			want: "https://127.0.0.1:9000/default?secure=true",
		},
		{
			name: "Preserve query parameters",
			fields: fields{
				Endpoint: "clickhouse://127.0.0.1:9000?secure=true&foo=bar",
			},
			wantChOptions: ChOptions{
				Secure: true,
			},
			args: args{},
			want: "clickhouse://127.0.0.1:9000/default?foo=bar&secure=true",
		},
		{
			name: "Parse clickhouse settings",
			fields: fields{
				Endpoint: "https://127.0.0.1:9000?secure=true&dial_timeout=30s&compress=lz4",
			},
			wantChOptions: ChOptions{
				Secure:      true,
				DialTimeout: 30 * time.Second,
				Compress:    clickhouse.CompressionLZ4,
			},
			args: args{},
			want: "https://127.0.0.1:9000/default?compress=lz4&dial_timeout=30s&secure=true",
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
			args: args{},
			want: "clickhouse://127.0.0.1:9000/default?foo=bar&secure=true",
		},
		{
			name: "support replace database in DSN to default database",
			fields: fields{
				Endpoint: "tcp://127.0.0.1:9000/otel",
			},
			args: args{
				database: defaultDatabase,
			},
			want: "tcp://127.0.0.1:9000/default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Endpoint:         tt.fields.Endpoint,
				Username:         tt.fields.Username,
				Password:         configopaque.String(tt.fields.Password),
				Database:         tt.fields.Database,
				ConnectionParams: tt.fields.ConnectionParams,
			}
			got, err := cfg.buildDSN(tt.args.database)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr, "buildDSN(%v)", tt.args.database)
			} else {
				// Validate DSN
				opts, err := clickhouse.ParseDSN(got)
				assert.Nil(t, err)
				assert.Equalf(t, tt.wantChOptions.Secure, opts.TLS != nil, "TLSConfig is not nil")
				assert.Equalf(t, tt.wantChOptions.DialTimeout, opts.DialTimeout, "DialTimeout is not nil")
				if tt.wantChOptions.Compress != 0 {
					assert.Equalf(t, tt.wantChOptions.Compress, opts.Compression.Method, "Compress is not nil")
				}
				assert.Equalf(t, tt.want, got, "buildDSN(%v)", tt.args.database)
			}

		})
	}
}
