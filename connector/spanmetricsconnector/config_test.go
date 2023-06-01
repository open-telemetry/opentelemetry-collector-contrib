// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanmetricsconnector

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultMethod := "GET"
	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id:       component.NewIDWithName(typeStr, "default"),
			expected: createDefaultConfig(),
		},
		{
			id:       component.NewIDWithName(typeStr, "default_explicit_histogram"),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(typeStr, "full"),
			expected: &Config{
				AggregationTemporality: delta,
				Dimensions: []Dimension{
					{Name: "http.method", Default: &defaultMethod},
					{Name: "http.status_code", Default: (*string)(nil)},
				},
				DimensionsCacheSize:  1500,
				MetricsFlushInterval: 30 * time.Second,
				Histogram: HistogramConfig{
					Unit: metrics.Seconds,
					Explicit: &ExplicitHistogramConfig{
						Buckets: []time.Duration{
							10 * time.Millisecond,
							100 * time.Millisecond,
							250 * time.Millisecond,
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "exponential_histogram"),
			expected: &Config{
				AggregationTemporality: cumulative,
				DimensionsCacheSize:    1000,
				MetricsFlushInterval:   15 * time.Second,
				Histogram: HistogramConfig{
					Unit: metrics.Milliseconds,
					Exponential: &ExponentialHistogramConfig{
						MaxSize: 10,
					},
				},
			},
		},
		{
			id:           component.NewIDWithName(typeStr, "exponential_and_explicit_histogram"),
			errorMessage: "use either `explicit` or `exponential` buckets histogram",
		},
		{
			id:           component.NewIDWithName(typeStr, "invalid_histogram_unit"),
			errorMessage: "unknown Unit \"h\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			err = component.UnmarshalConfig(sub, cfg)

			if tt.expected == nil {
				err = multierr.Append(err, component.ValidateConfig(cfg))
				assert.ErrorContains(t, err, tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestGetAggregationTemporality(t *testing.T) {
	cfg := &Config{AggregationTemporality: delta}
	assert.Equal(t, pmetric.AggregationTemporalityDelta, cfg.GetAggregationTemporality())

	cfg = &Config{AggregationTemporality: cumulative}
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, cfg.GetAggregationTemporality())

	cfg = &Config{}
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, cfg.GetAggregationTemporality())
}

func TestValidateDimensions(t *testing.T) {
	for _, tc := range []struct {
		name        string
		dimensions  []Dimension
		expectedErr string
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
			name: "duplicate additional dimensions",
			dimensions: []Dimension{
				{Name: "service_name"},
				{Name: "service_name"},
			},
			expectedErr: "duplicate dimension name service_name",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDimensions(tc.dimensions)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
