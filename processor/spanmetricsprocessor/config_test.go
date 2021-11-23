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
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
)

func TestLoadConfig(t *testing.T) {
	defaultMethod := "GET"
	testcases := []struct {
		configFile                  string
		wantMetricsExporter         string
		wantLatencyHistogramBuckets []time.Duration
		wantDimensions              []Dimension
	}{
		{configFile: "config-2-pipelines.yaml", wantMetricsExporter: "prometheus"},
		{configFile: "config-3-pipelines.yaml", wantMetricsExporter: "otlp/spanmetrics"},
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
		},
	}
	for _, tc := range testcases {
		t.Run(tc.configFile, func(t *testing.T) {
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
			cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", tc.configFile), factories)

			// Verify
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t,
				&Config{
					ProcessorSettings:       config.NewProcessorSettings(config.NewComponentID(typeStr)),
					MetricsExporter:         tc.wantMetricsExporter,
					LatencyHistogramBuckets: tc.wantLatencyHistogramBuckets,
					Dimensions:              tc.wantDimensions,
				},
				cfg.Processors[config.NewComponentID(typeStr)],
			)
		})
	}
}
