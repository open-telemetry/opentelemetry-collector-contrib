// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanmetricsconnector

import (
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultMethod := http.MethodGet
	defaultMaxPerDatapoint := 5
	customTimestampCacheSize := 123
	tests := []struct {
		id              component.ID
		expected        component.Config
		errorMessage    string
		extraAssertions func(config *Config)
	}{
		{
			id:       component.NewIDWithName(metadata.Type, "default"),
			expected: createDefaultConfig(),
		},
		{
			id:       component.NewIDWithName(metadata.Type, "default_explicit_histogram"),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "full"),
			expected: &Config{
				AggregationTemporality: delta,
				Dimensions: []Dimension{
					{Name: "http.method", Default: &defaultMethod},
					{Name: "http.status_code", Default: (*string)(nil)},
				},
				Namespace:                DefaultNamespace,
				ResourceMetricsCacheSize: 1600,
				MetricsFlushInterval:     30 * time.Second,
				Exemplars: ExemplarsConfig{
					Enabled: true,
				},
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
			id: component.NewIDWithName(metadata.Type, "exponential_histogram"),
			expected: &Config{
				Namespace:                DefaultNamespace,
				AggregationTemporality:   cumulative,
				ResourceMetricsCacheSize: defaultResourceMetricsCacheSize,
				MetricsFlushInterval:     60 * time.Second,
				Histogram: HistogramConfig{
					Unit: metrics.Milliseconds,
					Exponential: &ExponentialHistogramConfig{
						MaxSize: 10,
					},
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "exponential_and_explicit_histogram"),
			errorMessage: "use either `explicit` or `exponential` buckets histogram",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_histogram_unit"),
			errorMessage: "unknown Unit \"h\"",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_metrics_expiration"),
			errorMessage: "the duration should be positive",
		},
		{
			id: component.NewIDWithName(metadata.Type, "exemplars_enabled"),
			expected: &Config{
				AggregationTemporality:   "AGGREGATION_TEMPORALITY_CUMULATIVE",
				ResourceMetricsCacheSize: defaultResourceMetricsCacheSize,
				MetricsFlushInterval:     60 * time.Second,
				Histogram:                HistogramConfig{Disable: false, Unit: defaultUnit},
				Exemplars:                ExemplarsConfig{Enabled: true},
				Namespace:                DefaultNamespace,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "exemplars_enabled_with_max_per_datapoint"),
			expected: &Config{
				AggregationTemporality:   "AGGREGATION_TEMPORALITY_CUMULATIVE",
				ResourceMetricsCacheSize: defaultResourceMetricsCacheSize,
				MetricsFlushInterval:     60 * time.Second,
				Histogram:                HistogramConfig{Disable: false, Unit: defaultUnit},
				Exemplars:                ExemplarsConfig{Enabled: true, MaxPerDataPoint: &defaultMaxPerDatapoint},
				Namespace:                DefaultNamespace,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "resource_metrics_key_attributes"),
			expected: &Config{
				AggregationTemporality:       "AGGREGATION_TEMPORALITY_CUMULATIVE",
				ResourceMetricsCacheSize:     defaultResourceMetricsCacheSize,
				ResourceMetricsKeyAttributes: []string{"service.name", "telemetry.sdk.language", "telemetry.sdk.name"},
				MetricsFlushInterval:         60 * time.Second,
				Histogram:                    HistogramConfig{Disable: false, Unit: defaultUnit},
				Namespace:                    DefaultNamespace,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom_delta_timestamp_cache_size"),
			expected: &Config{
				AggregationTemporality:   "AGGREGATION_TEMPORALITY_DELTA",
				TimestampCacheSize:       &customTimestampCacheSize,
				ResourceMetricsCacheSize: defaultResourceMetricsCacheSize,
				MetricsFlushInterval:     60 * time.Second,
				Histogram:                HistogramConfig{Disable: false, Unit: defaultUnit},
				Namespace:                DefaultNamespace,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "default_delta_timestamp_cache_size"),
			expected: &Config{
				AggregationTemporality:   "AGGREGATION_TEMPORALITY_DELTA",
				ResourceMetricsCacheSize: defaultResourceMetricsCacheSize,
				MetricsFlushInterval:     60 * time.Second,
				Histogram:                HistogramConfig{Disable: false, Unit: defaultUnit},
				Namespace:                DefaultNamespace,
			},
			extraAssertions: func(config *Config) {
				assert.Equal(t, defaultDeltaTimestampCacheSize, config.GetDeltaTimestampCacheSize())
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "invalid_delta_timestamp_cache_size"),
			errorMessage: "invalid delta timestamp cache size: 0, the maximum number of the items in the cache should be positive",
		},
		{
			id: component.NewIDWithName(metadata.Type, "separate_calls_and_duration_dimensions"),
			expected: &Config{
				AggregationTemporality: "AGGREGATION_TEMPORALITY_CUMULATIVE",
				Histogram:              HistogramConfig{Disable: false, Unit: defaultUnit, Dimensions: []Dimension{{Name: "http.status_code", Default: (*string)(nil)}}},
				Dimensions: []Dimension{
					{Name: "http.method", Default: &defaultMethod},
				},
				CallsDimensions: []Dimension{
					{Name: "http.url", Default: (*string)(nil)},
				},
				ResourceMetricsCacheSize: defaultResourceMetricsCacheSize,
				MetricsFlushInterval:     60 * time.Second,
				Namespace:                DefaultNamespace,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			err = sub.Unmarshal(cfg)

			if tt.expected == nil {
				err = errors.Join(err, xconfmap.Validate(cfg))
				assert.ErrorContains(t, err, tt.errorMessage)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
			if tt.extraAssertions != nil {
				tt.extraAssertions(cfg.(*Config))
			}
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

func TestValidateEventDimensions(t *testing.T) {
	for _, tc := range []struct {
		enabled     bool
		name        string
		dimensions  []Dimension
		expectedErr string
	}{
		{
			enabled:    false,
			name:       "disabled - no additional dimensions",
			dimensions: []Dimension{},
		},
		{
			enabled:     true,
			name:        "enabled - no additional dimensions",
			dimensions:  []Dimension{},
			expectedErr: "no dimensions configured for events",
		},
		{
			enabled:    true,
			name:       "enabled - no duplicate dimensions",
			dimensions: []Dimension{{Name: "exception_type"}},
		},
		{
			enabled: true,
			name:    "enabled - duplicate dimensions",
			dimensions: []Dimension{
				{Name: "exception_type"},
				{Name: "exception_type"},
			},
			expectedErr: "duplicate dimension name exception_type",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEventDimensions(tc.enabled, tc.dimensions)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectedErr string
	}{
		{
			name: "valid config",
			config: Config{
				ResourceMetricsCacheSize: 1000,
				MetricsFlushInterval:     60 * time.Second,
				Histogram: HistogramConfig{
					Explicit: &ExplicitHistogramConfig{
						Buckets: []time.Duration{10 * time.Millisecond},
					},
				},
			},
		},
		{
			name: "invalid metrics flush interval",
			config: Config{
				ResourceMetricsCacheSize: 1000,
				MetricsFlushInterval:     -1 * time.Second,
			},
			expectedErr: "invalid metrics_flush_interval: -1s, the duration should be positive",
		},
		{
			name: "invalid metrics expiration",
			config: Config{
				ResourceMetricsCacheSize: 1000,
				MetricsFlushInterval:     60 * time.Second,
				MetricsExpiration:        -1 * time.Second,
			},
			expectedErr: "invalid metrics_expiration: -1s, the duration should be positive",
		},
		{
			name: "invalid delta timestamp cache size",
			config: Config{
				ResourceMetricsCacheSize: 1000,
				MetricsFlushInterval:     60 * time.Second,
				AggregationTemporality:   delta,
				TimestampCacheSize:       new(int), // zero value
			},
			expectedErr: "invalid delta timestamp cache size: 0, the maximum number of the items in the cache should be positive",
		},
		{
			name: "invalid aggregation cardinality limit",
			config: Config{
				ResourceMetricsCacheSize:    1000,
				MetricsFlushInterval:        60 * time.Second,
				AggregationCardinalityLimit: -1,
			},
			expectedErr: "invalid aggregation_cardinality_limit: -1, the limit should be positive",
		},
		{
			name: "both explicit and exponential histogram",
			config: Config{
				ResourceMetricsCacheSize: 1000,
				MetricsFlushInterval:     60 * time.Second,
				Histogram: HistogramConfig{
					Explicit: &ExplicitHistogramConfig{
						Buckets: []time.Duration{10 * time.Millisecond},
					},
					Exponential: &ExponentialHistogramConfig{
						MaxSize: 10,
					},
				},
			},
			expectedErr: "use either `explicit` or `exponential` buckets histogram",
		},
		{
			name: "duplicate dimension name",
			config: Config{
				ResourceMetricsCacheSize: 1000,
				MetricsFlushInterval:     60 * time.Second,
				Dimensions: []Dimension{
					{Name: "service.name"},
				},
			},
			expectedErr: "failed validating dimensions: duplicate dimension name service.name",
		},
		{
			name: "events enabled with no dimensions",
			config: Config{
				ResourceMetricsCacheSize: 1000,
				MetricsFlushInterval:     60 * time.Second,
				Events: EventsConfig{
					Enabled: true,
				},
			},
			expectedErr: "failed validating event dimensions: no dimensions configured for events",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
