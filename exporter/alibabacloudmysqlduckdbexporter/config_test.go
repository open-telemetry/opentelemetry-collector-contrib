// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudmysqlduckdbexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudmysqlduckdbexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	c := cfg.(*Config)
	assert.Equal(t, "root:password@tcp(localhost:3306)/otel", c.Endpoint)
	assert.Equal(t, "otel", c.Database)
	assert.Equal(t, "otel_logs", c.LogsTableName)
	assert.Equal(t, "otel_traces", c.TracesTableName)
	assert.Equal(t, "otel_metrics", c.MetricsTableName)
	assert.True(t, c.CreateSchema)
	assert.Equal(t, 72*time.Hour, c.TTL)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &Config{
				Endpoint: "root:password@tcp(localhost:3306)/otel",
				Database: "otel",
			},
			wantErr: false,
		},
		{
			name: "missing endpoint",
			cfg: &Config{
				Database: "otel",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuildDSN(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{
			name:     "without query params",
			endpoint: "root:pass@tcp(localhost:3306)/otel",
			want:     "root:pass@tcp(localhost:3306)/otel?parseTime=true",
		},
		{
			name:     "with query params",
			endpoint: "root:pass@tcp(localhost:3306)/otel?tls=true",
			want:     "root:pass@tcp(localhost:3306)/otel?tls=true&parseTime=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Endpoint: tt.endpoint}
			assert.Equal(t, tt.want, cfg.buildDSN())
		})
	}
}

func TestMetricTableNames(t *testing.T) {
	cfg := &Config{MetricsTableName: "my_metrics"}
	assert.Equal(t, "my_metrics_gauge", cfg.gaugeTableName())
	assert.Equal(t, "my_metrics_sum", cfg.sumTableName())
	assert.Equal(t, "my_metrics_histogram", cfg.histogramTableName())
	assert.Equal(t, "my_metrics_summary", cfg.summaryTableName())
}
