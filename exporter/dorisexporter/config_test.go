// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultCfg := createDefaultConfig()
	defaultCfg.(*Config).Endpoint = "http://localhost:8030"
	defaultCfg.(*Config).MySQLEndpoint = "localhost:9030"
	err = defaultCfg.(*Config).Validate()
	require.NoError(t, err)

	httpClientConfig := confighttp.NewDefaultClientConfig()
	httpClientConfig.Timeout = 5 * time.Second
	httpClientConfig.Endpoint = "http://localhost:8030"
	httpClientConfig.Headers = map[string]configopaque.String{
		"max_filter_ratio": "0.1",
		"strict_mode":      "true",
		"group_commit":     "async_mode",
	}

	fullCfg := &Config{
		ClientConfig: httpClientConfig,
		BackOffConfig: configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     5 * time.Second,
			MaxInterval:         30 * time.Second,
			MaxElapsedTime:      300 * time.Second,
			RandomizationFactor: backoff.DefaultRandomizationFactor,
			Multiplier:          backoff.DefaultMultiplier,
		},
		QueueSettings: exporterhelper.QueueBatchConfig{
			Enabled:      true,
			NumConsumers: 10,
			QueueSize:    1000,
			Sizer:        exporterhelper.RequestSizerTypeRequests,
		},
		Table: Table{
			Logs:    "otel_logs",
			Traces:  "otel_traces",
			Metrics: "otel_metrics",
		},
		Database:            "otel",
		Username:            "admin",
		Password:            configopaque.String("admin"),
		CreateSchema:        true,
		MySQLEndpoint:       "localhost:9030",
		HistoryDays:         0,
		CreateHistoryDays:   0,
		ReplicationNum:      2,
		TimeZone:            "Asia/Shanghai",
		LogResponse:         true,
		LabelPrefix:         "otel",
		LogProgressInterval: 5,
	}
	err = fullCfg.Validate()
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: defaultCfg,
		},
		{
			id:       component.NewIDWithName(metadata.Type, "full"),
			expected: fullCfg,
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
