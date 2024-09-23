// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

type apiResponse struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	ErrorType v1.ErrorType    `json:"errorType"`
	Error     string          `json:"error"`
	Warnings  []string        `json:"warnings,omitempty"`
}

type scrapePoolsData struct {
	ScrapePools []string `json:"scrapePools"`
}

func TestPrometheusAPIServer(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: metricSet, useOpenMetrics: false},
			},
			normalizedName: false,
			validateFunc:   verifyMetrics,
		},
	}

	ctx := context.Background()
	mp, cfg, err := setupMockPrometheus(targets...)
	require.Nilf(t, err, "Failed to create Prometheus config: %v", err)
	defer mp.Close()

	endpoints := []string{"localhost:9090", "localhost:9091"}
	for _, endpoint := range endpoints {
		require.NoError(t, err)
		receiver := newPrometheusReceiver(receivertest.NewNopSettings(), &Config{
			PrometheusConfig: (*PromConfig)(cfg),
			APIServer: &APIServer{
				Enabled: true,
				ServerConfig: confighttp.ServerConfig{
					Endpoint: endpoint,
				},
			},
		}, new(consumertest.MetricsSink))

		require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))
		t.Cleanup(func() {
			require.NoError(t, receiver.Shutdown(ctx))
			response, err := callAPI(endpoint, "/scrape_pools")
			require.Error(t, err)
			require.Nil(t, response)
		})
	}

	mp.wg.Wait()

	for _, endpoint := range endpoints {
		testScrapePools(t, endpoint)
		testTargets(t, endpoint)
		testTargetsMetadata(t, endpoint)
		testPrometheusConfig(t, endpoint)
		testMetricsEndpoint(t, endpoint)
	}
}

func callAPI(endpoint, path string) (*apiResponse, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/api/v1%s", endpoint, path))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResponse apiResponse
	err = json.NewDecoder(resp.Body).Decode(&apiResponse)
	if err != nil {
		return nil, err
	}

	if apiResponse.Status != "success" {
		return nil, fmt.Errorf("API call failed: %s", apiResponse.Error)
	}

	return &apiResponse, nil
}

func testScrapePools(t *testing.T, endpoint string) {
	scrapePoolsResponse, err := callAPI(endpoint, "/scrape_pools")
	assert.NoError(t, err)
	var scrapePoolsData scrapePoolsData
	json.Unmarshal([]byte(scrapePoolsResponse.Data), &scrapePoolsData)
	assert.NotNil(t, scrapePoolsData)
	assert.NotEmpty(t, scrapePoolsData.ScrapePools)
	assert.Contains(t, scrapePoolsData.ScrapePools, "target1")
}

func testTargets(t *testing.T, endpoint string) {
	targetsResponse, err := callAPI(endpoint, "/targets")
	assert.NoError(t, err)
	var targets v1.TargetsResult
	json.Unmarshal([]byte(targetsResponse.Data), &targets)
	assert.NotNil(t, targets)
	assert.NotNil(t, targets.Active)
	for _, target := range targets.Active {
		assert.NotNil(t, target)
		assert.NotEmpty(t, target.DiscoveredLabels)
		assert.NotEmpty(t, target.Labels)
	}
}

func testTargetsMetadata(t *testing.T, endpoint string) {
	targetsMetadataResponse, err := callAPI(endpoint, "/targets/metadata?match_target={job=\"target1\"}")
	assert.NoError(t, err)
	assert.NotNil(t, targetsMetadataResponse)

	var metricMetadataResult []v1.MetricMetadata
	json.Unmarshal([]byte(targetsMetadataResponse.Data), &metricMetadataResult)
	assert.NotNil(t, metricMetadataResult)
	for _, metricMetadata := range metricMetadataResult {
		assert.NotNil(t, metricMetadata)
		assert.NotNil(t, metricMetadata.Target)
		assert.NotEmpty(t, metricMetadata.Metric)
		assert.NotEmpty(t, metricMetadata.Type)
	}
}

func testPrometheusConfig(t *testing.T, endpoint string) {
	prometheusConfigResponse, err := callAPI(endpoint, "/status/config")
	assert.NoError(t, err)
	var prometheusConfigResult v1.ConfigResult
	json.Unmarshal([]byte(prometheusConfigResponse.Data), &prometheusConfigResult)
	assert.NotNil(t, prometheusConfigResult)
	assert.NotNil(t, prometheusConfigResult.YAML)
	prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, prometheusConfig)
}

func testMetricsEndpoint(t *testing.T, endpoint string) {
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", endpoint))
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "prometheus_target_scrape_pools_total")
}
