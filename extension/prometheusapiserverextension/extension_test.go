package prometheusapiserverextension

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/service/extensions"
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

type prometheusConfigData struct {
	PrometheusConfigYAML string `json:"yaml"`
}

func TestPrometheusAPIServerExtension(t *testing.T) {
	ctx, _ := context.WithCancel(context.Background())

	mockReceiver, discoveryManagerCancelFunc, err := createMockReceiver()
	assert.NoError(t, err)
	mockReceiverList := make(map[string]*prometheusReceiver)
	mockReceiverList["mockReceiver"] = mockReceiver

	ext := &prometheusAPIServerExtension{
		config:              &Config{},
		settings:            extension.CreateSettings{},
		prometheusReceivers: mockReceiverList,
	}
	extensions, err := extensions.New(ctx, extensions.Settings{}, extensions.Config{})
	assert.NoError(t, err)

	mockHost := &mockServiceHost{
		serviceExtensions: extensions,
	}

	err = ext.Start(ctx, mockHost)
	assert.NoError(t, err)

	err = ext.RegisterPrometheusReceiverComponents(mockReceiver.name, mockReceiver.serverConfig, mockReceiver.prometheusConfig, mockReceiver.scrapeManager, mockReceiver.registerer)
	assert.NoError(t, err)

	time.Sleep(5 * time.Second)

	testScrapePools(t)
	testTargets(t)
	testTargetsMetadata(t)
	testPrometheusConfig(t)
	testMetricsEndpoint(t)

	err = ext.Shutdown(ctx)
	assert.NoError(t, err)

	_, err = callAPI("/scrape_pools")
	assert.Error(t, err)

	mockReceiver.scrapeManager.Stop()
	discoveryManagerCancelFunc()
}

func testScrapePools(t *testing.T) {
	scrapePoolsResponse, err := callAPI("/scrape_pools")
	assert.NoError(t, err)
	var scrapePoolsData scrapePoolsData
	json.Unmarshal([]byte(scrapePoolsResponse.Data), &scrapePoolsData)
	assert.NotNil(t, scrapePoolsData)
	assert.NotEmpty(t, scrapePoolsData.ScrapePools)
	assert.Contains(t, scrapePoolsData.ScrapePools, "my_job")
}

func testTargets(t *testing.T) {
	targetsResponse, err := callAPI("/targets")
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

func testTargetsMetadata(t *testing.T) {
	targetsMetadataResponse, err := callAPI("/targets/metadata?match_target={job=\"my_job\"}")
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

func testPrometheusConfig(t *testing.T) {
	prometheusConfigResponse, err := callAPI("/status/config")
	assert.NoError(t, err)
	var prometheusConfigResult v1.ConfigResult
	json.Unmarshal([]byte(prometheusConfigResponse.Data), &prometheusConfigResult)
	assert.NotNil(t, prometheusConfigResult)
	assert.NotNil(t, prometheusConfigResult.YAML)
	prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, prometheusConfig)
}

func testMetricsEndpoint(t *testing.T) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:9090/metrics"))
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "prometheus_target_scrape_pools_total")
}

func callAPI(path string) (*apiResponse, error) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:9090/api/v1%s", path))
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

func createMockReceiver() (*prometheusReceiver, context.CancelFunc, error) {

	// Create and run the discoveryManager
	registerer := prometheus.WrapRegistererWith(
		prometheus.Labels{"receiver": "mockreceiver"},
		prometheus.DefaultRegisterer,
	)
	sdMetrics, err := discovery.CreateAndRegisterSDMetrics(registerer)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	discoveryManager := discovery.NewManager(ctx, nil, registerer, sdMetrics)
	go func() {
		discoveryManager.Run()
	}()

	// Create and run the scrapeManager
	scrapeManager, err := scrape.NewManager(&scrape.Options{
		PassMetadataInContext: true,
		ExtraMetrics:          false,
		HTTPClientOptions:     []commonconfig.HTTPClientOption{},
	}, nil, nil, registerer)
	if err != nil {
		return nil, cancel, err
	}

	go func() {
		scrapeManager.Run(discoveryManager.SyncCh())
	}()

	// Create the Prometheus config from the test YAML file
	filePath := "testdata/config.yaml"
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, cancel, err
	}
	cfg, err := config.Load(string(content), true, nil)
	if err != nil {
		return nil, cancel, err
	}

	// Apply the config to the scrapeManager and discoveryManager
	if err := scrapeManager.ApplyConfig((*config.Config)(cfg)); err != nil {
		return nil, cancel, err
	}
	discoveryCfg := make(map[string]discovery.Configs)
	for _, scrapeConfig := range cfg.ScrapeConfigs {
		discoveryCfg[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
	}
	if err := discoveryManager.ApplyConfig(discoveryCfg); err != nil {
		return nil, cancel, err
	}

	mockReceiver := &prometheusReceiver{
		name:             "mockReceiver",
		serverConfig:     confighttp.ServerConfig{
			Endpoint: 			"localhost:9090",
		},
		prometheusConfig: cfg,
		scrapeManager:    scrapeManager,
		registerer:       registerer,
	}

	return mockReceiver, cancel, nil
}

type mockServiceHost struct {
	serviceExtensions *extensions.Extensions
}

func (host *mockServiceHost) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
	return nil
}

func (host *mockServiceHost) GetExtensions() map[component.ID]component.Component {
	return host.serviceExtensions.GetExtensions()
}

func (host *mockServiceHost) GetExporters() map[component.DataType]map[component.ID]component.Component {
	return nil
}
