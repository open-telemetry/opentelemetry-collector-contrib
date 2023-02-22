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

package spanmetricsconnector

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

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
	factories.Connectors[typeStr] = NewFactory()
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
