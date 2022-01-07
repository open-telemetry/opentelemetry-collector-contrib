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
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
)

func TestLoadConfig(t *testing.T) {
	defaultMethod := "GET"
	defaultRegion := "us-east-1"
	testcases := []struct {
		configFile                      string
		wantMetricsExporter             string
		wantLatencyHistogramBuckets     []time.Duration
		wantDimensions                  []Dimension
		wantDimensionsCacheSize         int
		wantResourceAttributes          []Dimension
		wantResourceAttributesCacheSize int
		wantAggregationTemporality      string
	}{
		{
			configFile:                      "config-2-pipelines.yaml",
			wantMetricsExporter:             "prometheus",
			wantAggregationTemporality:      cumulative,
			wantDimensionsCacheSize:         500,
			wantResourceAttributesCacheSize: 300,
		},
		{
			configFile:                      "config-3-pipelines.yaml",
			wantMetricsExporter:             "otlp/spanmetrics",
			wantAggregationTemporality:      cumulative,
			wantDimensionsCacheSize:         defaultDimensionsCacheSize,
			wantResourceAttributesCacheSize: defaultResourceAttributesCacheSize,
		},
		{
			configFile:          "config-full.yaml",
			wantMetricsExporter: "otlp/spanmetrics",
			wantLatencyHistogramBuckets: []time.Duration{
				100 * time.Microsecond,
				1 * time.Millisecond,
				2 * time.Millisecond,
				6 * time.Millisecond,
				10 * time.Millisecond,
				100 * time.Millisecond,
				250 * time.Millisecond,
			},
			wantDimensions: []Dimension{
				{"http.method", &defaultMethod},
				{"http.status_code", nil},
			},
			wantDimensionsCacheSize: 1500,
			wantResourceAttributes: []Dimension{
				{"region", &defaultRegion},
				{"host_id", nil},
			},
			wantResourceAttributesCacheSize: 3000,
			wantAggregationTemporality:      delta,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.configFile, func(t *testing.T) {
			t.Parallel()
			// Prepare
			factories, err := componenttest.NopFactories()
			require.NoError(t, err)

			factories.Receivers["otlp"] = otlpreceiver.NewFactory()
			factories.Receivers["jaeger"] = jaegerreceiver.NewFactory()

			factories.Processors[typeStr] = NewFactory()
			factories.Processors["batch"] = batchprocessor.NewFactory()

			factories.Exporters["otlp"] = otlpexporter.NewFactory()
			factories.Exporters["prometheus"] = prometheusexporter.NewFactory()
			factories.Exporters["jaeger"] = jaegerexporter.NewFactory()

			// Test
			cfg, err := servicetest.LoadConfigAndValidate(path.Join(".", "testdata", tc.configFile), factories)

			// Verify
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t,
				&Config{
					ProcessorSettings:           config.NewProcessorSettings(config.NewComponentID(typeStr)),
					MetricsExporter:             tc.wantMetricsExporter,
					LatencyHistogramBuckets:     tc.wantLatencyHistogramBuckets,
					Dimensions:                  tc.wantDimensions,
					DimensionsCacheSize:         tc.wantDimensionsCacheSize,
					ResourceAttributes:          tc.wantResourceAttributes,
					ResourceAttributesCacheSize: tc.wantResourceAttributesCacheSize,
					AggregationTemporality:      tc.wantAggregationTemporality,
				},
				cfg.Processors[config.NewComponentID(typeStr)],
			)
		})
	}
}

func TestGetAggregationTemporality(t *testing.T) {
	cfg := &Config{AggregationTemporality: delta}
	assert.Equal(t, pdata.MetricAggregationTemporalityDelta, cfg.GetAggregationTemporality())

	cfg = &Config{AggregationTemporality: cumulative}
	assert.Equal(t, pdata.MetricAggregationTemporalityCumulative, cfg.GetAggregationTemporality())

	cfg = &Config{}
	assert.Equal(t, pdata.MetricAggregationTemporalityCumulative, cfg.GetAggregationTemporality())
}
