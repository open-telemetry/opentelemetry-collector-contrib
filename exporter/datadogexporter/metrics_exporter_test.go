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
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
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
	logger := zap.NewNop()

	// The client should have been created correctly
	exp, err := newMetricsExporter(logger, cfg)
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

	logger := zap.NewNop()

	exp, err := newMetricsExporter(logger, cfg)

	require.NoError(t, err)

	metrics := []datadog.Metric{
		newGauge(
			"metric_name",
			0,
			0,
			[]string{"key2:val2"},
		),
	}

	exp.processMetrics(metrics)

	assert.Equal(t, "test-host", *metrics[0].Host)
	assert.Equal(t, "test.metric_name", *metrics[0].Metric)
	assert.ElementsMatch(t,
		[]string{"key:val", "env:test_env", "key2:val2"},
		metrics[0].Tags,
	)

}
