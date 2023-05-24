// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanmetricsprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	defaultMethod := "GET"
	tests := []struct {
		name     string
		id       component.ID
		expected component.Config
	}{
		{
			name: "configuration with dimensions size cache",
			id:   component.NewIDWithName(metadata.Type, "dimensions"),
			expected: &Config{
				MetricsExporter:        "prometheus",
				AggregationTemporality: cumulative,
				DimensionsCacheSize:    500,
				MetricsFlushInterval:   15 * time.Second,
			},
		},
		{
			name: "configuration with aggregation temporality",
			id:   component.NewIDWithName(metadata.Type, "temp"),
			expected: &Config{
				MetricsExporter:        "otlp/spanmetrics",
				AggregationTemporality: cumulative,
				DimensionsCacheSize:    defaultDimensionsCacheSize,
				MetricsFlushInterval:   15 * time.Second,
			},
		},
		{
			name: "configuration with all available parameters",
			id:   component.NewIDWithName(metadata.Type, "full"),
			expected: &Config{
				MetricsExporter:        "otlp/spanmetrics",
				AggregationTemporality: delta,
				DimensionsCacheSize:    1500,
				MetricsFlushInterval:   30 * time.Second,
				LatencyHistogramBuckets: []time.Duration{
					100 * time.Microsecond,
					1 * time.Millisecond,
					2 * time.Millisecond,
					6 * time.Millisecond,
					10 * time.Millisecond,
					100 * time.Millisecond,
					250 * time.Millisecond,
				},
				Dimensions: []Dimension{
					{"http.method", &defaultMethod},
					{"http.status_code", nil},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)

			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateDimensions(t *testing.T) {
	for _, tc := range []struct {
		name              string
		dimensions        []Dimension
		expectedErr       string
		skipSanitizeLabel bool
	}{
		{
			name:       "no additional dimensions",
			dimensions: []Dimension{},
		},
		{
			name: "no duplicate dimensions",
			dimensions: []Dimension{
				{Name: "http.service_name"},
				{Name: "http.status_code"},
			},
		},
		{
			name: "duplicate dimension with reserved labels",
			dimensions: []Dimension{
				{Name: "service.name"},
			},
			expectedErr: "duplicate dimension name service.name",
		},
		{
			name: "duplicate dimension with reserved labels after sanitization",
			dimensions: []Dimension{
				{Name: "service_name"},
			},
			expectedErr: "duplicate dimension name service_name",
		},
		{
			name: "duplicate additional dimensions",
			dimensions: []Dimension{
				{Name: "service_name"},
				{Name: "service_name"},
			},
			expectedErr: "duplicate dimension name service_name",
		},
		{
			name: "duplicate additional dimensions after sanitization",
			dimensions: []Dimension{
				{Name: "http.status_code"},
				{Name: "http!status_code"},
			},
			expectedErr: "duplicate dimension name http_status_code after sanitization",
		},
		{
			name: "we skip the case if the dimension name is the same after sanitization",
			dimensions: []Dimension{
				{Name: "http_status_code"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.skipSanitizeLabel = false
			err := validateDimensions(tc.dimensions, tc.skipSanitizeLabel)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
