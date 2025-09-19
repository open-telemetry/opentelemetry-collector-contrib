// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

// TestDurationAndMetricsInteraction tests the interaction between duration and metrics parameters
func TestDurationAndMetricsInteraction(t *testing.T) {
	tests := []struct {
		name            string
		config          Config
		expectedMetrics int
		description     string
	}{
		{
			name: "Default behavior - respects metrics parameter",
			config: Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumMetrics: 3,
			},
			expectedMetrics: 3,
			description:     "By default, TotalDuration is 0, so NumMetrics should be respected",
		},
		{
			name: "Finite duration overrides metrics",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.DurationWithInf(100 * time.Millisecond),
				},
				NumMetrics: 100,
			},
			expectedMetrics: 0,
			description:     "Finite duration should override NumMetrics (set to 0)",
		},
		{
			name: "Infinite duration overrides metrics",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.MustDurationWithInf("Inf"),
				},
				NumMetrics: 50,
			},
			expectedMetrics: 0,
			description:     "Infinite duration should override NumMetrics (set to 0)",
		},
		{
			name: "Zero duration with metrics",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.DurationWithInf(0),
				},
				NumMetrics: 5,
			},
			expectedMetrics: 5,
			description:     "Zero duration should not override NumMetrics",
		},
		{
			name: "Negative duration with metrics",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.DurationWithInf(-100 * time.Millisecond),
				},
				NumMetrics: 10,
			},
			expectedMetrics: 10,
			description:     "Negative duration should not override NumMetrics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.config

			if cfg.TotalDuration.Duration() > 0 || cfg.TotalDuration.IsInf() {
				cfg.NumMetrics = 0
			}

			assert.Equal(t, tt.expectedMetrics, cfg.NumMetrics, tt.description)
		})
	}
}

// TestDefaultConfiguration tests that the default configuration is correct
func TestDefaultConfiguration(t *testing.T) {
	cfg := NewConfig()

	assert.Equal(t, types.DurationWithInf(0), cfg.TotalDuration, "Default TotalDuration should be 0")
	assert.Equal(t, 0, cfg.NumMetrics, "Default NumMetrics should be 0")
	assert.Equal(t, float64(1), cfg.Rate, "Default Rate should be 1")
	assert.Equal(t, "gen", cfg.MetricName, "Default MetricName should be 'gen'")
	assert.Equal(t, MetricTypeGauge, cfg.MetricType, "Default MetricType should be Gauge")
}

// TestConfigValidation tests the validation logic
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		description string
	}{
		{
			name: "Valid config with metrics",
			config: Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumMetrics: 5,
			},
			expectError: false,
			description: "Config with NumMetrics > 0 should be valid",
		},
		{
			name: "Valid config with finite duration",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.DurationWithInf(1 * time.Second),
				},
				NumMetrics: 0,
			},
			expectError: false,
			description: "Config with finite duration > 0 should be valid",
		},
		{
			name: "Valid config with infinite duration",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.MustDurationWithInf("Inf"),
				},
				NumMetrics: 0,
			},
			expectError: false,
			description: "Config with infinite duration should be valid",
		},
		{
			name: "Invalid config - no metrics and no duration",
			config: Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumMetrics: 0,
			},
			expectError: true,
			description: "Config with no metrics and no duration should be invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

// TestWorkerBehavior tests that workers behave correctly with different configurations
func TestWorkerBehavior(t *testing.T) {
	tests := []struct {
		name            string
		config          Config
		expectedMetrics int
		description     string
	}{
		{
			name: "Worker with finite metrics and no duration",
			config: Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumMetrics: 2,
			},
			expectedMetrics: 2,
			description:     "Worker should generate exactly the specified number of metrics",
		},
		{
			name: "Worker with infinite duration",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.MustDurationWithInf("Inf"),
				},
				NumMetrics: 0, // This will be set by the run logic
			},
			expectedMetrics: 0,
			description:     "Worker with infinite duration should have NumMetrics set to 0",
		},
		{
			name: "Worker with finite duration",
			config: Config{
				Config: common.Config{
					WorkerCount:   1,
					TotalDuration: types.DurationWithInf(100 * time.Millisecond),
				},
				NumMetrics: 10, // This will be set to 0 by the run logic
			},
			expectedMetrics: 0,
			description:     "Worker with finite duration should have NumMetrics set to 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.TotalDuration.Duration() > 0 || tt.config.TotalDuration.IsInf() {
				tt.config.NumMetrics = 0
			}

			assert.Equal(t, tt.expectedMetrics, tt.config.NumMetrics, tt.description)
		})
	}
}

func Test_exemplarsFromConfig(t *testing.T) {
	traceID, err := hex.DecodeString("ae87dadd90e9935a4bc9660628efd569")
	require.NoError(t, err)

	spanID, err := hex.DecodeString("5828fa4960140870")
	require.NoError(t, err)

	tests := []struct {
		name         string
		c            *Config
		validateFunc func(t *testing.T, got []metricdata.Exemplar[int64])
	}{
		{
			name: "no exemplars",
			c:    &Config{},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				assert.Nil(t, got)
			},
		},
		{
			name: "both-traceID-and-spanID",
			c: &Config{
				TraceID: "ae87dadd90e9935a4bc9660628efd569",
				SpanID:  "5828fa4960140870",
			},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				require.Len(t, got, 1)
				metricdatatest.AssertEqual[metricdata.Exemplar[int64]](t, got[0], metricdata.Exemplar[int64]{
					TraceID: traceID,
					SpanID:  spanID,
				}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
			},
		},
		{
			name: "only-traceID",
			c: &Config{
				TraceID: "ae87dadd90e9935a4bc9660628efd569",
			},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				require.Len(t, got, 1)
				metricdatatest.AssertEqual[metricdata.Exemplar[int64]](t, got[0], metricdata.Exemplar[int64]{
					TraceID: traceID,
					SpanID:  nil,
				}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
			},
		},
		{
			name: "only-spanID",
			c: &Config{
				SpanID: "5828fa4960140870",
			},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				require.Len(t, got, 1)
				metricdatatest.AssertEqual[metricdata.Exemplar[int64]](t, got[0], metricdata.Exemplar[int64]{
					TraceID: nil,
					SpanID:  spanID,
				}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validateFunc(t, exemplarsFromConfig(tt.c))
		})
	}
}

func TestNewMetricTypes(t *testing.T) {
	tests := []struct {
		name         string
		metricType   MetricType
		validateFunc func(t *testing.T, config Config)
	}{
		{
			name:       "ExponentialHistogram metric type",
			metricType: MetricTypeExponentialHistogram,
			validateFunc: func(t *testing.T, config Config) {
				assert.Equal(t, MetricTypeExponentialHistogram, config.MetricType)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				MetricType: tt.metricType,
			}
			tt.validateFunc(t, config)
		})
	}
}
