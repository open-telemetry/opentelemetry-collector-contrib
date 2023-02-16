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
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
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

func TestLoadConnectorConfig(t *testing.T) {
	defaultMethod := "GET"
	// Need to set this gate to load connector configs
	require.NoError(t, featuregate.GlobalRegistry().Set("service.connectors", true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set("service.connectors", false))
	}()

	// Prepare
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factories.Receivers["nop"] = receivertest.NewNopFactory()
	factories.Connectors[typeStr] = NewConnectorFactory()
	factories.Exporters["nop"] = exportertest.NewNopFactory()

	// Test
	simpleCfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-simplest-connector.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, simpleCfg)
	assert.Equal(t,
		&Config{
			AggregationTemporality: cumulative,
			DimensionsCacheSize:    defaultDimensionsCacheSize,
			skipSanitizeLabel:      dropSanitizationGate.IsEnabled(),
			MetricsFlushInterval:   15 * time.Second,
		},
		simpleCfg.Connectors[component.NewID(typeStr)],
	)

	fullCfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-full-connector.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, fullCfg)
	assert.Equal(t,
		&Config{
			LatencyHistogramBuckets: []time.Duration{100000, 1000000, 2000000, 6000000, 10000000, 100000000, 250000000},
			Dimensions: []Dimension{
				{Name: "http.method", Default: &defaultMethod},
				{Name: "http.status_code", Default: (*string)(nil)},
			},
			AggregationTemporality: delta,
			DimensionsCacheSize:    1500,
			skipSanitizeLabel:      dropSanitizationGate.IsEnabled(),
			MetricsFlushInterval:   30 * time.Second,
		},
		fullCfg.Connectors[component.NewID(typeStr)],
	)
}

func TestGetAggregationTemporality(t *testing.T) {
	cfg := &Config{AggregationTemporality: delta}
	assert.Equal(t, pmetric.AggregationTemporalityDelta, cfg.GetAggregationTemporality())

	cfg = &Config{AggregationTemporality: cumulative}
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, cfg.GetAggregationTemporality())

	cfg = &Config{}
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, cfg.GetAggregationTemporality())
}
