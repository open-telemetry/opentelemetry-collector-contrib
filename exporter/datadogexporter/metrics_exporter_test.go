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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.uber.org/zap"
)

func TestNewExporterValid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{\"valid\": true}"))
	}))
	defer ts.Close()

	cfg := &Config{}
	cfg.API.Key = "ddog_32_characters_long_api_key1"
	cfg.Metrics.TCPAddr.Endpoint = ts.URL
	logger := zap.NewNop()

	// The client should have been created correctly
	exp, err := newMetricsExporter(logger, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestNewExporterInvalid(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{\"valid\": false}"))
	}))
	defer ts.Close()

	cfg := &Config{}
	cfg.API.Key = "ddog_32_characters_long_api_key1"
	cfg.Metrics.TCPAddr.Endpoint = ts.URL
	logger := zap.NewNop()

	// An error should be raised
	exp, err := newMetricsExporter(logger, cfg)
	assert.Equal(t,
		errors.New("provided Datadog API key is invalid: ***************************_key1"),
		err,
	)
	assert.Nil(t, exp)
}

func TestNewExporterValidateError(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()

	cfg := &Config{}
	cfg.API.Key = "ddog_32_characters_long_api_key1"
	cfg.Metrics.TCPAddr.Endpoint = ts.URL
	logger := zap.NewNop()

	// The client should have been created correctly
	// with the error being ignored
	exp, err := newMetricsExporter(logger, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestProcessMetrics(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()

	cfg := &Config{
		TagsConfig: TagsConfig{
			Hostname: "test_host",
			Env:      "test_env",
			Tags:     []string{"key:val"},
		},
		Metrics: MetricsConfig{
			TCPAddr:   confignet.TCPAddr{Endpoint: ts.URL},
			Namespace: "test.",
		},
	}
	logger := zap.NewNop()

	exp, err := newMetricsExporter(logger, cfg)

	require.NoError(t, err)

	var series Series
	series.Add(NewGauge(
		"original_host",
		"metric_name",
		0,
		0,
		[]string{"key2:val2"},
	))

	exp.processMetrics(&series)

	assert.Equal(t, "test_host", *series.metrics[0].Host)
	assert.Equal(t, "test.metric_name", *series.metrics[0].Metric)
	assert.ElementsMatch(t,
		[]string{"key:val", "env:test_env", "key2:val2"},
		series.metrics[0].Tags,
	)

}
