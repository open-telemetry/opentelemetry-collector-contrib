// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datadogexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/testutils"
)

func TestNewExporter(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	cfg := &config.Config{
		API: config.APIConfig{
			Key: "ddog_32_characters_long_api_key1",
		},
		Metrics: config.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
		},
	}

	cfg.Sanitize()
	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	// The client should have been created correctly
	exp, err := newMetricsExporter(params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestProcessMetrics(t *testing.T) {
	server := testutils.DatadogServerMock()
	defer server.Close()

	cfg := &config.Config{
		API: config.APIConfig{
			Key: "ddog_32_characters_long_api_key1",
		},
		// Global tags should be ignored and sent as metadata
		TagsConfig: config.TagsConfig{
			Hostname: "test-host",
			Env:      "test_env",
			Tags:     []string{"key:val"},
		},
		Metrics: config.MetricsConfig{
			Namespace: "test.",
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
		},
	}
	cfg.Sanitize()

	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exp, err := newMetricsExporter(params, cfg)

	require.NoError(t, err)

	ms := []datadog.Metric{
		metrics.NewGauge(
			"metric_name",
			0,
			0,
			[]string{"key2:val2"},
		),
	}

	exp.processMetrics(ms)

	assert.Equal(t, "test-host", *ms[0].Host)
	assert.Equal(t, "test.metric_name", *ms[0].Metric)
	assert.ElementsMatch(t,
		[]string{"key2:val2"},
		ms[0].Tags,
	)

}
