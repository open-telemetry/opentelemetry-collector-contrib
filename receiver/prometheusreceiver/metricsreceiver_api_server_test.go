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
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	api_v1 "github.com/prometheus/prometheus/web/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
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

	endpointsToReceivers := map[string]*pReceiver{
		"localhost:9090": nil,
		"localhost:9091": nil,
	}
	for endpoint := range endpointsToReceivers {
		ctx := context.Background()
		mp, cfg, err := setupMockPrometheus(targets...)
		require.NoErrorf(t, err, "Failed to create Prometheus config: %v", err)
		defer mp.Close()

		require.NoError(t, err)
		receiver := newPrometheusReceiver(receivertest.NewNopSettings(metadata.Type), &Config{
			PrometheusConfig: cfg,
			APIServer: &APIServer{
				Enabled: true,
				ServerConfig: confighttp.ServerConfig{
					Endpoint: endpoint,
				},
			},
		}, new(consumertest.MetricsSink))
		endpointsToReceivers[endpoint] = receiver

		require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))
		t.Cleanup(func() {
			require.NoError(t, receiver.Shutdown(ctx))
			response, err := callAPI(endpoint, "/scrape_pools")
			require.Error(t, err)
			require.Nil(t, response)
		})

		mp.wg.Wait()
	}

	for endpoint, receiver := range endpointsToReceivers {
		testScrapePools(t, endpoint)
		testTargets(t, endpoint)
		testTargetsMetadata(t, endpoint)
		testPrometheusConfig(t, endpoint, receiver)
		testMetricsEndpoint(t, endpoint)
		testRuntimeInfo(t, endpoint)
		testBuildInfo(t, endpoint)
		testFlags(t, endpoint)
	}
}

func callAPI(endpoint, path string) (*apiResponse, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/api/v1%s", endpoint, path))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response apiResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	if response.Status != "success" {
		return nil, fmt.Errorf("API call failed: %s", response.Error)
	}

	return &response, nil
}

func testScrapePools(t *testing.T, endpoint string) {
	scrapePoolsResponse, err := callAPI(endpoint, "/scrape_pools")
	assert.NoError(t, err)
	var scrapePools scrapePoolsData
	err = json.Unmarshal([]byte(scrapePoolsResponse.Data), &scrapePools)
	assert.NoError(t, err)
	assert.NotNil(t, scrapePools)
	assert.NotEmpty(t, scrapePools.ScrapePools)
	assert.Contains(t, scrapePools.ScrapePools, "target1")
}

func testTargets(t *testing.T, endpoint string) {
	targetsResponse, err := callAPI(endpoint, "/targets")
	assert.NoError(t, err)
	var targets v1.TargetsResult
	err = json.Unmarshal([]byte(targetsResponse.Data), &targets)
	assert.NoError(t, err)
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
	err = json.Unmarshal([]byte(targetsMetadataResponse.Data), &metricMetadataResult)
	assert.NoError(t, err)
	assert.NotNil(t, metricMetadataResult)
	for _, metricMetadata := range metricMetadataResult {
		assert.NotNil(t, metricMetadata)
		assert.NotNil(t, metricMetadata.Target)
		assert.NotEmpty(t, metricMetadata.Metric)
		assert.NotEmpty(t, metricMetadata.Type)
	}
}

func testPrometheusConfig(t *testing.T, endpoint string, receiver *pReceiver) {
	prometheusConfigResponse, err := callAPI(endpoint, "/status/config")
	assert.NoError(t, err)
	var prometheusConfigResult v1.ConfigResult
	err = json.Unmarshal([]byte(prometheusConfigResponse.Data), &prometheusConfigResult)
	assert.NoError(t, err)
	assert.NotNil(t, prometheusConfigResult)
	assert.NotNil(t, prometheusConfigResult.YAML)
	prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
	assert.NoError(t, err)
	assert.NotNil(t, prometheusConfig)

	// Modify the Prometheus config
	newScrapeInterval := model.Duration(30 * time.Second)
	receiver.cfg.PrometheusConfig.GlobalConfig.ScrapeInterval = newScrapeInterval
	receiver.cfg.PrometheusConfig.ScrapeConfigs[0].ScrapeInterval = newScrapeInterval

	// Call the API again and check if the change exists in the returned config
	newPrometheusConfigResponse, err := callAPI(endpoint, "/status/config")
	assert.NoError(t, err)
	var newPrometheusConfigResult v1.ConfigResult
	err = json.Unmarshal([]byte(newPrometheusConfigResponse.Data), &newPrometheusConfigResult)
	assert.NoError(t, err)
	assert.NotNil(t, newPrometheusConfigResult)
	assert.NotNil(t, newPrometheusConfigResult.YAML)
	newPrometheusConfig, err := config.Load(newPrometheusConfigResult.YAML, nil)
	assert.NoError(t, err)
	assert.NotNil(t, newPrometheusConfig)
	assert.Equal(t, newScrapeInterval, newPrometheusConfig.GlobalConfig.ScrapeInterval)
	assert.Equal(t, newScrapeInterval, newPrometheusConfig.ScrapeConfigs[0].ScrapeInterval)

	// Ensure the new config is different from the old one
	assert.NotEqual(t, prometheusConfig, newPrometheusConfig)
}

func testRuntimeInfo(t *testing.T, endpoint string) {
	prometheusConfigResponse, err := callAPI(endpoint, "/status/runtimeinfo")
	assert.NoError(t, err)
	var runtimeInfo api_v1.RuntimeInfo
	err = json.Unmarshal([]byte(prometheusConfigResponse.Data), &runtimeInfo)
	assert.NoError(t, err)
	assert.NotNil(t, runtimeInfo)
	assert.NotEmpty(t, runtimeInfo.GoroutineCount)
	assert.NotEmpty(t, runtimeInfo.GOMAXPROCS)
	assert.NotEmpty(t, runtimeInfo.GOMEMLIMIT)
}

func testBuildInfo(t *testing.T, endpoint string) {
	prometheusConfigResponse, err := callAPI(endpoint, "/status/buildinfo")
	assert.NoError(t, err)

	var prometheusVersion api_v1.PrometheusVersion
	err = json.Unmarshal([]byte(prometheusConfigResponse.Data), &prometheusVersion)
	assert.NoError(t, err)
	assert.NotNil(t, prometheusVersion)
	assert.NotEmpty(t, prometheusVersion.GoVersion)
}

func testFlags(t *testing.T, endpoint string) {
	prometheusConfigResponse, err := callAPI(endpoint, "/status/flags")
	assert.NoError(t, err)
	var flagsMap map[string]string
	err = json.Unmarshal([]byte(prometheusConfigResponse.Data), &flagsMap)
	assert.NoError(t, err)
	assert.NotNil(t, flagsMap)
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
