// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"context"
	"strings"
	"testing"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

const renameMetric = `
# HELP DCGM_FI_DEV_GPU_TEMP GPU temperature (in C).
# TYPE DCGM_FI_DEV_GPU_TEMP gauge
DCGM_FI_DEV_GPU_TEMP{gpu="0",UUID="uuid",device="nvidia0",modelName="NVIDIA A10G",Hostname="hostname",DCGM_FI_DRIVER_VERSION="470.182.03",container="main",namespace="kube-system",pod="fullname-hash"} 65
# HELP DCGM_FI_DEV_GPU_UTIL GPU utilization (in %).
# TYPE DCGM_FI_DEV_GPU_UTIL gauge
DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="uuid",device="nvidia0",modelName="NVIDIA A10G",Hostname="hostname",DCGM_FI_DRIVER_VERSION="470.182.03",container="main",namespace="kube-system",pod="fullname-hash"} 100
`

const (
	dummyInstanceID   = "i-0000000000"
	dummyClusterName  = "cluster-name"
	dummyInstanceType = "instance-type"
)

type mockHostInfoProvider struct{}

func (m mockHostInfoProvider) GetClusterName() string {
	return dummyClusterName
}

func (m mockHostInfoProvider) GetInstanceID() string {
	return dummyInstanceID
}

func (m mockHostInfoProvider) GetInstanceType() string {
	return dummyInstanceType
}

type mockConsumer struct {
	t        *testing.T
	called   *bool
	expected map[string]struct {
		value  float64
		labels map[string]string
	}
}

func (m mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m mockConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	assert.Equal(m.t, 1, md.ResourceMetrics().Len())

	scrapedMetricCnt := 0
	scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < scopeMetrics.Len(); i++ {
		metric := scopeMetrics.At(i)

		// skip prometheus metadata metrics including "up"
		if !strings.HasPrefix(metric.Name(), "DCGM") {
			continue
		}
		metadata, ok := m.expected[metric.Name()]
		assert.True(m.t, ok)
		assert.Equal(m.t, metadata.value, metric.Gauge().DataPoints().At(0).DoubleValue())
		for k, v := range metadata.labels {
			lv, found := metric.Gauge().DataPoints().At(0).Attributes().Get(k)
			assert.True(m.t, found)
			assert.Equal(m.t, v, lv.AsString())
		}
		scrapedMetricCnt++
	}
	assert.Equal(m.t, len(m.expected), scrapedMetricCnt)
	*m.called = true
	return nil
}

func TestNewDcgmScraperEndToEnd(t *testing.T) {
	expected := map[string]struct {
		value  float64
		labels map[string]string
	}{
		"DCGM_FI_DEV_GPU_TEMP": {
			value: 65,
			labels: map[string]string{
				ci.NodeNameKey:      "hostname",
				ci.K8sNamespace:     "kube-system",
				ci.ClusterNameKey:   dummyClusterName,
				ci.InstanceID:       dummyInstanceID,
				ci.InstanceType:     dummyInstanceType,
				ci.FullPodNameKey:   "fullname-hash",
				ci.PodNameKey:       "fullname-hash",
				ci.ContainerNamekey: "main",
				ci.GpuDevice:        "nvidia0",
			},
		},
		"DCGM_FI_DEV_GPU_UTIL": {
			value: 100,
			labels: map[string]string{
				ci.NodeNameKey:      "hostname",
				ci.K8sNamespace:     "kube-system",
				ci.ClusterNameKey:   dummyClusterName,
				ci.InstanceID:       dummyInstanceID,
				ci.InstanceType:     dummyInstanceType,
				ci.FullPodNameKey:   "fullname-hash",
				ci.PodNameKey:       "fullname-hash",
				ci.ContainerNamekey: "main",
				ci.GpuDevice:        "nvidia0",
			},
		},
	}

	consumerCalled := false
	mConsumer := mockConsumer{
		t:        t,
		called:   &consumerCalled,
		expected: expected,
	}

	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	scraper, err := prometheusscraper.NewSimplePrometheusScraper(prometheusscraper.SimplePrometheusScraperOpts{
		Ctx:               context.TODO(),
		TelemetrySettings: settings,
		Consumer:          mConsumer,
		Host:              componenttest.NewNopHost(),
		HostInfoProvider:  mockHostInfoProvider{},
		ScraperConfigs:    GetScraperConfig(mockHostInfoProvider{}),
		Logger:            settings.Logger,
	})
	assert.NoError(t, err)
	assert.Equal(t, mockHostInfoProvider{}, scraper.HostInfoProvider)

	// build up a new PR
	promFactory := prometheusreceiver.NewFactory()

	targets := []*mocks.TestData{
		{
			Name: "dcgm",
			Pages: []mocks.MockPrometheusResponse{
				{Code: 200, Data: renameMetric},
			},
		},
	}
	mp, cfg, err := mocks.SetupMockPrometheus(targets...)
	assert.NoError(t, err)

	scrapeConfig := scraper.ScraperConfigs
	scrapeConfig.ScrapeProtocols = cfg.ScrapeConfigs[0].ScrapeProtocols
	scrapeConfig.ScrapeInterval = cfg.ScrapeConfigs[0].ScrapeInterval
	scrapeConfig.ScrapeTimeout = cfg.ScrapeConfigs[0].ScrapeInterval
	scrapeConfig.Scheme = "http"
	scrapeConfig.MetricsPath = cfg.ScrapeConfigs[0].MetricsPath
	scrapeConfig.HTTPClientConfig = configutil.HTTPClientConfig{
		TLSConfig: configutil.TLSConfig{
			InsecureSkipVerify: true,
		},
	}
	scrapeConfig.ServiceDiscoveryConfigs = discovery.Configs{
		// using dummy static config to avoid service discovery initialization
		&discovery.StaticConfig{
			{
				Targets: []model.LabelSet{
					{
						model.AddressLabel: model.LabelValue(strings.Split(mp.Srv.URL, "http://")[1]),
					},
				},
			},
		},
	}

	promConfig := prometheusreceiver.Config{
		PrometheusConfig: &prometheusreceiver.PromConfig{
			ScrapeConfigs: []*config.ScrapeConfig{scrapeConfig},
		},
	}

	// replace the prom receiver
	params := receiver.Settings{
		TelemetrySettings: scraper.Settings,
	}
	scraper.PrometheusReceiver, err = promFactory.CreateMetrics(scraper.Ctx, params, &promConfig, mConsumer)

	assert.NoError(t, err)
	assert.NotNil(t, mp)
	defer mp.Close()

	// perform a single scrape, this will kick off the scraper process for additional scrapes
	scraper.GetMetrics()

	t.Cleanup(func() {
		scraper.Shutdown()
	})

	// wait for 2 scrapes, one initiated by us, another by the new scraper process
	mp.Wg.Wait()
	mp.Wg.Wait()
	// make sure the consumer is called at scraping interval
	assert.True(t, consumerCalled)
}

func TestDcgmScraperJobName(t *testing.T) {
	// needs to start with containerInsights
	assert.True(t, strings.HasPrefix(jobName, "containerInsightsDCGMExporterScraper"))
}
