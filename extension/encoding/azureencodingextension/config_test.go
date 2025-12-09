// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension"

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/traces"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				Format: unmarshaler.FormatEventHub,
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "incorrect_format"),
			expected:    nil,
			expectedErr: "unsupported format: \"json\"",
		},
		{
			id: component.NewIDWithName(metadata.Type, "blobstorage_format"),
			expected: &Config{
				Format: unmarshaler.FormatBlobStorage,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "metrics_time_formats"),
			expected: &Config{
				Format: unmarshaler.FormatEventHub,
				Metrics: metrics.MetricsConfig{
					TimeFormats: []string{"01/02/2006 15:04:05", "2006-01-02T15:04:05Z"},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "metrics_aggregations"),
			expected: &Config{
				Format: unmarshaler.FormatEventHub,
				Metrics: metrics.MetricsConfig{
					Aggregations: []metrics.MetricAggregation{
						metrics.AggregationCount,
						metrics.AggregationTotal,
						metrics.AggregationAverage,
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "metrics_aggregations_unknown"),
			expected:    nil,
			expectedErr: "metrics: invalid aggregation \"unknown\"",
		},
		{
			id: component.NewIDWithName(metadata.Type, "traces_time_formats"),
			expected: &Config{
				Format: unmarshaler.FormatEventHub,
				Traces: traces.TracesConfig{
					TimeFormats: []string{"01/02/2006 15:04:05", "2006-01-02T15:04:05Z"},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "logs_time_formats"),
			expected: &Config{
				Format: unmarshaler.FormatEventHub,
				Logs: logs.LogsConfig{
					TimeFormats: []string{"01/02/2006 15:04:05", "2006-01-02T15:04:05Z"},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "logs_include_categories"),
			expected: &Config{
				Format: unmarshaler.FormatEventHub,
				Logs: logs.LogsConfig{
					IncludeCategories: []string{"AzureCdnAccessLog", "FrontDoorAccessLog"},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "logs_exclude_categories"),
			expected: &Config{
				Format: unmarshaler.FormatEventHub,
				Logs: logs.LogsConfig{
					ExcludeCategories: []string{"AppServiceAppLogs", "AppServiceAuditLogs"},
				},
			},
		},
	}

	for _, tt := range tests {
		name := strings.ReplaceAll(tt.id.String(), "/", "_")
		t.Run(name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = xconfmap.Validate(cfg)
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}
