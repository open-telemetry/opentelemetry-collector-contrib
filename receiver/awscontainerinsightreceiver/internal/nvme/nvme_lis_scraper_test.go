// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nvme

import (
	"context"
	"strings"
	"testing"

	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	_ "github.com/prometheus/prometheus/discovery/install" // init() registers service discovery impl.

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

const lisRenameMetric = `
# HELP aws_ec2_instance_store_csi_read_ops_total Total number of read operations
# TYPE aws_ec2_instance_store_csi_read_ops_total counter
aws_ec2_instance_store_csi_read_ops_total{instance_id="i-0131bee5395cc4317",volume_id="vol-0281cf921f3dbb69b"} 55592
# HELP aws_ec2_instance_store_csi_read_seconds_total Total time spent on read operations
# TYPE aws_ec2_instance_store_csi_read_seconds_total counter
aws_ec2_instance_store_csi_read_seconds_total{instance_id="i-0131bee5395cc4317",volume_id="vol-0281cf921f3dbb69b"} 34.52
# HELP aws_ec2_instance_store_csi_write_ops_total Total number of write operations
# TYPE aws_ec2_instance_store_csi_write_ops_total counter
aws_ec2_instance_store_csi_write_ops_total{instance_id="i-0131bee5395cc4317",volume_id="vol-0281cf921f3dbb69b"} 12345
# HELP aws_ec2_instance_store_csi_write_bytes_total Total bytes written
# TYPE aws_ec2_instance_store_csi_write_bytes_total counter
aws_ec2_instance_store_csi_write_bytes_total{instance_id="i-0131bee5395cc4317",volume_id="vol-0281cf921f3dbb69b"} 987654321
`

type mockLisConsumer struct {
	t        *testing.T
	called   *bool
	expected map[string]struct {
		value  float64
		labels map[string]string
	}
}

func (m mockLisConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m mockLisConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	assert.Equal(m.t, 1, md.ResourceMetrics().Len())

	scrapedMetricCnt := 0
	scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < scopeMetrics.Len(); i++ {
		metric := scopeMetrics.At(i)
		// skip prometheus metadata metrics including "up"
		if !strings.HasPrefix(metric.Name(), "aws_ec2_instance_store_csi") {
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

func TestGetLisScraperConfig(t *testing.T) {
	mockProvider := mockHostInfoProvider{}

	config := GetLisScraperConfig(mockProvider)

	assert.Equal(t, lisJobName, config.JobName)
	assert.Equal(t, lisScraperMetricsPath, config.MetricsPath)
	assert.Len(t, config.ServiceDiscoveryConfigs, 1)
	assert.NotEmpty(t, config.MetricRelabelConfigs)

	// Verify service discovery config
	sdConfig := config.ServiceDiscoveryConfigs[0]
	assert.NotNil(t, sdConfig)

	// Verify metric relabel configs
	relabelConfigs := config.MetricRelabelConfigs
	assert.NotEmpty(t, relabelConfigs)

	// Check that the first relabel config keeps LIS metrics
	keepConfig := relabelConfigs[0]
	assert.Equal(t, relabel.Keep, keepConfig.Action)
	assert.Equal(t, "aws_ec2_instance_store_csi_.*", keepConfig.Regex.String())
}

func TestLisMetricRelabelConfig(t *testing.T) {
	mockProvider := mockHostInfoProvider{}
	relabelConfigs := getLisMetricRelabelConfig(mockProvider)

	assert.NotEmpty(t, relabelConfigs)

	// Test keep config for LIS metrics
	keepConfig := relabelConfigs[0]
	assert.Equal(t, relabel.Keep, keepConfig.Action)
	assert.Equal(t, "aws_ec2_instance_store_csi_.*", keepConfig.Regex.String())

	// Test drop config for histogram metrics
	dropConfig := relabelConfigs[1]
	assert.Equal(t, relabel.Drop, dropConfig.Action)
	assert.Equal(t, ".*_bucket|.*_sum|.*_count.*", dropConfig.Regex.String())

	// Test cluster name injection
	found := false
	for _, config := range relabelConfigs {
		if config.TargetLabel == ci.ClusterNameKey {
			assert.Equal(t, relabel.Replace, config.Action)
			assert.Equal(t, dummyClusterName, config.Replacement)
			found = true
			break
		}
	}
	assert.True(t, found, "ClusterName relabel config not found")
}

func TestNewLisNVMEScraperEndToEnd(t *testing.T) {
	t.Setenv("HOST_NAME", dummyNodeName)
	expected := map[string]struct {
		value  float64
		labels map[string]string
	}{
		"aws_ec2_instance_store_csi_read_seconds_total": {
			value: 34.52,
			labels: map[string]string{
				ci.NodeNameKey:    "hostname",
				ci.ClusterNameKey: dummyClusterName,
				ci.InstanceID:     "i-0131bee5395cc4317",
				ci.VolumeID:       "vol-0281cf921f3dbb69b",
			},
		},
		"aws_ec2_instance_store_csi_read_ops_total": {
			value: 55592,
			labels: map[string]string{
				ci.NodeNameKey:    "hostname",
				ci.ClusterNameKey: dummyClusterName,
				ci.InstanceID:     "i-0131bee5395cc4317",
				ci.VolumeID:       "vol-0281cf921f3dbb69b",
			},
		},
		"aws_ec2_instance_store_csi_write_ops_total": {
			value: 12345,
			labels: map[string]string{
				ci.NodeNameKey:    "hostname",
				ci.ClusterNameKey: dummyClusterName,
				ci.InstanceID:     "i-0131bee5395cc4317",
				ci.VolumeID:       "vol-0281cf921f3dbb69b",
			},
		},
		"aws_ec2_instance_store_csi_write_bytes_total": {
			value: 987654321,
			labels: map[string]string{
				ci.NodeNameKey:    "hostname",
				ci.ClusterNameKey: dummyClusterName,
				ci.InstanceID:     "i-0131bee5395cc4317",
				ci.VolumeID:       "vol-0281cf921f3dbb69b",
			},
		},
	}

	consumerCalled := false
	mConsumer := mockLisConsumer{
		t:        t,
		called:   &consumerCalled,
		expected: expected,
	}

	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	// Create the scraper struct directly without calling the prometheus factory
	// with production configs that use kubernetes SD (which can't be marshaled to YAML).
	// We'll replace the PrometheusReceiver with one created using the mock config.
	scraperConfig := GetLisScraperConfig(mockHostInfoProvider{})
	scraper := &prometheusscraper.SimplePrometheusScraper{
		Ctx:              t.Context(),
		Settings:         settings,
		Host:             componenttest.NewNopHost(),
		HostInfoProvider: mockHostInfoProvider{},
		ScraperConfigs:   scraperConfig,
	}
	assert.Equal(t, mockHostInfoProvider{}, scraper.HostInfoProvider)

	// build up a new PR
	promFactory := prometheusreceiver.NewFactory()

	targets := []*mocks.TestData{
		{
			Name: "nvme-lis",
			Pages: []mocks.MockPrometheusResponse{
				{Code: 200, Data: lisRenameMetric},
			},
		},
	}
	mp, cfg, err := mocks.SetupMockPrometheus(targets...)
	assert.NoError(t, err)

	// Apply the metric relabel configs from the production config to the mock config
	if len(cfg.ScrapeConfigs) > 0 {
		cfg.ScrapeConfigs[0].MetricRelabelConfigs = scraperConfig.MetricRelabelConfigs
	}

	// Use the config returned by SetupMockPrometheus directly, as it's already
	// properly loaded via promcfg.Load() with all discovery types registered.
	promConfig := prometheusreceiver.Config{
		PrometheusConfig: cfg,
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

func TestLisNvmeScraperJobName(t *testing.T) {
	// needs to start with containerInsights
	assert.True(t, strings.HasPrefix(lisJobName, "containerInsights"))

	mockProvider := mockHostInfoProvider{}
	config := GetLisScraperConfig(mockProvider)
	assert.Equal(t, "containerInsightsNVMeLISScraper", config.JobName)
}
