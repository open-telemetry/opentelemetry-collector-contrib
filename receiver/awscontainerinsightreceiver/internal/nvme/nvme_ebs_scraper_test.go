// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nvme

import (
	"context"
	"strings"
	"testing"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
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
# HELP aws_ebs_csi_read_ops_total The total number of completed read operations.
# TYPE aws_ebs_csi_read_ops_total counter
aws_ebs_csi_read_ops_total{instance_id="i-0131bee5395cc4317",volume_id="vol-0281cf921f3dbb69b"} 55592
# HELP aws_ebs_csi_read_seconds_total The total time spent, in seconds, by all completed read operations.
# TYPE aws_ebs_csi_read_seconds_total counter
aws_ebs_csi_read_seconds_total{instance_id="i-0131bee5395cc4317",volume_id="vol-0281cf921f3dbb69b"} 34.52
`

const (
	dummyInstanceID   = "i-0000000000"
	dummyClusterName  = "cluster-name"
	dummyInstanceType = "instance-type"
	dummyNodeName     = "hostname"
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
		if !strings.HasPrefix(metric.Name(), "aws_ebs") {
			continue
		}
		metadata, ok := m.expected[metric.Name()]
		assert.True(m.t, ok)
		assert.Equal(m.t, metadata.value, metric.Sum().DataPoints().At(0).DoubleValue())
		for k, v := range metadata.labels {
			gauge := metric.Sum().DataPoints().At(0)
			m.t.Logf("%v", gauge)
			lv, found := metric.Sum().DataPoints().At(0).Attributes().Get(k)
			assert.True(m.t, found)
			assert.Equal(m.t, v, lv.AsString())
		}
		scrapedMetricCnt++
	}
	assert.Equal(m.t, len(m.expected), scrapedMetricCnt)
	*m.called = true
	return nil
}

func TestNewNVMEScraperEndToEnd(t *testing.T) {
	t.Setenv("HOST_NAME", dummyNodeName)
	expected := map[string]struct {
		value  float64
		labels map[string]string
	}{
		"aws_ebs_csi_read_seconds_total": {
			value: 34.52,
			labels: map[string]string{
				ci.NodeNameKey:    "hostname",
				ci.ClusterNameKey: dummyClusterName,
				ci.InstanceID:     "i-0131bee5395cc4317",
				ci.VolumeID:       "vol-0281cf921f3dbb69b",
			},
		},
		"aws_ebs_csi_read_ops_total": {
			value: 55592,
			labels: map[string]string{
				ci.NodeNameKey:    "hostname",
				ci.ClusterNameKey: dummyClusterName,
				ci.InstanceID:     "i-0131bee5395cc4317",
				ci.VolumeID:       "vol-0281cf921f3dbb69b",
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
		Ctx:               t.Context(),
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
			Name: "nvme",
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
		ID:                component.NewID(component.MustNewType("prometheus")),
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

func TestNvmeScraperJobName(t *testing.T) {
	// needs to start with containerInsights
	assert.True(t, strings.HasPrefix(jobName, "containerInsightsNVMeExporterScraper"))
}
