// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kueuescraper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightskueuereceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

const kueueMetrics = `
# HELP kueue_pending_workloads Number of pending workloads per cluster queue and status
# TYPE kueue_pending_workloads gauge
kueue_pending_workloads{queue="default"} 3
# HELP kueue_admitted_active_workloads The number of admitted workloads that are active per cluster queue
# TYPE kueue_admitted_active_workloads gauge
kueue_admitted_active_workloads{queue="default"} 5
`

type mockKueueConsumer struct {
	t                    *testing.T
	called               *bool
	pendingWorkloadCount *bool
	activeWorkloadCount  *bool
}

func (m mockKueueConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m mockKueueConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	assert.Equal(m.t, 1, md.ResourceMetrics().Len())

	scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < scopeMetrics.Len(); i++ {
		metric := scopeMetrics.At(i)
		switch metric.Name() {
		case "kueue_pending_workloads":
			assert.Equal(m.t, float64(3), metric.Gauge().DataPoints().At(0).DoubleValue())
			*m.pendingWorkloadCount = true
		case "kueue_admitted_active_workloads":
			assert.Equal(m.t, float64(5), metric.Gauge().DataPoints().At(0).DoubleValue())
			*m.activeWorkloadCount = true
		}
	}
	*m.called = true
	return nil
}

func TestNewKueuePrometheusScraperBadInputs(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	tests := []KueuePrometheusScraperOpts{
		{ // case: no consumer
			Ctx:               t.Context(),
			TelemetrySettings: settings,
			Consumer:          nil,
			Host:              componenttest.NewNopHost(),
			ClusterName:       "DummyCluster",
		},
		{ // case: no host
			Ctx:               t.Context(),
			TelemetrySettings: settings,
			Consumer:          mockKueueConsumer{},
			Host:              nil,
			ClusterName:       "DummyCluster",
		},
		{ // case: no cluster name
			Ctx:               t.Context(),
			TelemetrySettings: settings,
			Consumer:          mockKueueConsumer{},
			Host:              componenttest.NewNopHost(),
		},
	}

	for _, tt := range tests {
		scraper, err := NewKueuePrometheusScraper(tt)

		assert.Error(t, err)
		assert.Nil(t, scraper)
	}
}

func TestNewKueuePrometheusScraperEndToEnd(t *testing.T) {
	consumerCalled := false
	pendingWorkloadCount := false
	activeWorkloadCount := false

	mConsumer := mockKueueConsumer{
		t:                    t,
		called:               &consumerCalled,
		pendingWorkloadCount: &pendingWorkloadCount,
		activeWorkloadCount:  &activeWorkloadCount,
	}

	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	// Setup mock prometheus server
	targets := []*mocks.TestData{
		{
			Name: "kueue_prometheus",
			Pages: []mocks.MockPrometheusResponse{
				{Code: 200, Data: kueueMetrics},
			},
		},
	}
	mp, cfg, err := mocks.SetupMockPrometheus(targets...)
	assert.NoError(t, err)
	defer mp.Close()

	// Apply metric relabel configs from production config to mock config
	metricRelabelConfigs := GetKueueMetricRelabelConfigs("DummyCluster")
	cfg.ScrapeConfigs[0].MetricRelabelConfigs = metricRelabelConfigs

	// Create prometheus receiver with mock config
	promFactory := prometheusreceiver.NewFactory()
	promConfig := prometheusreceiver.Config{
		PrometheusConfig: (*prometheusreceiver.PromConfig)(cfg),
	}
	params := receiver.Settings{
		TelemetrySettings: settings,
		ID:                component.NewIDWithName(component.MustNewType("prometheus"), ""),
	}
	promReceiver, err := promFactory.CreateMetrics(t.Context(), params, &promConfig, mConsumer)
	assert.NoError(t, err)

	// Create scraper struct directly (bypass factory to avoid Reload() call)
	scraper := &KueuePrometheusScraper{
		ctx:                t.Context(),
		settings:           settings,
		host:               componenttest.NewNopHost(),
		clusterName:        "DummyCluster",
		prometheusReceiver: promReceiver,
	}

	// perform a single scrape, this will kick off the scraper process for additional scrapes
	scraper.GetMetrics()

	t.Cleanup(func() {
		scraper.Shutdown()
	})

	// wait for 2 scrapes, one initiated by us, another by the new scraper process
	mp.Wg.Wait()
	mp.Wg.Wait()

	// assert consumer was called and all metrics were processed
	assert.True(t, *mConsumer.called)
	assert.True(t, *mConsumer.pendingWorkloadCount)
	assert.True(t, *mConsumer.activeWorkloadCount)
}

func TestKueuePrometheusScraperJobName(t *testing.T) {
	// needs to start with containerInsights
	assert.Equal(t, "containerInsightsKueueMetricsScraper", kmJobName)
}
