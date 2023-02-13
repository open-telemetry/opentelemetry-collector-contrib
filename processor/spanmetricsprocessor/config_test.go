// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spanmetricsprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
			id:   component.NewIDWithName(typeStr, "dimensions"),
			expected: &Config{
				MetricsExporter:        "prometheus",
				AggregationTemporality: cumulative,
				DimensionsCacheSize:    500,
				MetricsFlushInterval:   15 * time.Second,
			},
		},
		{
			name: "configuration with aggregation temporality",
			id:   component.NewIDWithName(typeStr, "temp"),
			expected: &Config{
				MetricsExporter:        "otlp/spanmetrics",
				AggregationTemporality: cumulative,
				DimensionsCacheSize:    defaultDimensionsCacheSize,
				MetricsFlushInterval:   15 * time.Second,
			},
		},
		{
			name: "configuration with all available parameters",
			id:   component.NewIDWithName(typeStr, "full"),
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

func TestGetAggregationTemporality(t *testing.T) {
	cfg := &Config{AggregationTemporality: delta}
	assert.Equal(t, pmetric.AggregationTemporalityDelta, cfg.GetAggregationTemporality())

	cfg = &Config{AggregationTemporality: cumulative}
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, cfg.GetAggregationTemporality())

	cfg = &Config{}
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, cfg.GetAggregationTemporality())
}
