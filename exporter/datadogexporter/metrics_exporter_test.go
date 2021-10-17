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
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/translator"
	"gopkg.in/zorkian/go-datadog-api.v2"
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
			DeltaTTL: 3600,
			HistConfig: config.HistogramConfig{
				Mode:         string(translator.HistogramModeNoBuckets),
				SendCountSum: true,
			},
		},
	}
	params := componenttest.NewNopExporterCreateSettings()

	// The client should have been created correctly
	exp, err := newMetricsExporter(context.Background(), params, cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
	_ = exp.PushMetricsData(context.Background(), testutils.TestMetrics.Clone())
	assert.Equal(t, len(server.MetadataChan), 0)

	cfg.SendMetadata = true
	cfg.UseResourceMetadata = true
	_ = exp.PushMetricsData(context.Background(), testutils.TestMetrics.Clone())
	body := <-server.MetadataChan
	var recvMetadata metadata.HostMetadata
	err = json.Unmarshal(body, &recvMetadata)
	require.NoError(t, err)
	assert.Equal(t, recvMetadata.InternalHostname, "custom-hostname")
}

func TestDisableHostnameExporter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api/v1/validate" {
			rw.Header().Set("Content-Type", "application/json")
			rw.Write([]byte(`{"Valid": true}`))
			return
		} else if req.URL.Path != "/api/v1/series" {
			return
		}

		b, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			assert.NoError(t, err, "http server received malformed series payload")
		}
		defer req.Body.Close()

		var series struct {
			Series []datadog.Metric `json:"series,omitempty"`
		}
		if err := json.Unmarshal(b, &series); err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			assert.NoError(t, err, "http server received malformed series payload")
		}

		for _, metric := range series.Series {
			if metric.Host != nil {
				assert.Empty(t, *metric.Host)
			}
		}
		rw.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	cfg := config.Config{
		API: config.APIConfig{
			Key: "ddog_32_characters_long_api_key1",
		},
		DisableHostname: true,
		TagsConfig: config.TagsConfig{
			Hostname: "test-host",
		},
		Metrics: config.MetricsConfig{
			TCPAddr: confignet.TCPAddr{
				Endpoint: server.URL,
			},
			DeltaTTL: 3600,
			HistConfig: config.HistogramConfig{
				Mode:         string(translator.HistogramModeNoBuckets),
				SendCountSum: true,
			},
		},
	}

	params := componenttest.NewNopExporterCreateSettings()
	exporter, err := createMetricsExporter(context.Background(), params, &cfg)
	assert.NoError(t, err)
	defer exporter.Shutdown(context.Background())

	err = exporter.ConsumeMetrics(context.Background(), testutils.TestMetrics.Clone())
	assert.NoError(t, err)
}
