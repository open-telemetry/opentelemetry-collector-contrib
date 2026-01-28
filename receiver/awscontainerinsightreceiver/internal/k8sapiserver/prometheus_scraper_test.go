// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapiserver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	_ "github.com/prometheus/prometheus/discovery/install" // init() registers service discovery impl.
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

const renameMetric = `
# HELP http_go_threads Number of OS threads created
# TYPE http_go_threads gauge
http_go_threads 19
# HELP http_connected_total connected clients
# TYPE http_connected_total counter
http_connected_total{method="post",port="6380"} 15.0
# HELP redis_http_requests_total Redis connected clients
# TYPE redis_http_requests_total counter
redis_http_requests_total{method="post",port="6380"} 10.0
redis_http_requests_total{method="post",port="6381"} 12.0
# HELP rpc_duration_total RPC clients
# TYPE rpc_duration_total counter
rpc_duration_total{method="post",port="6380"} 100.0
rpc_duration_total{method="post",port="6381"} 120.0
`

type mockConsumer struct {
	t             *testing.T
	up            *bool
	httpConnected *bool
}

func (m mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m mockConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	assert.Equal(m.t, 1, md.ResourceMetrics().Len())

	scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	fmt.Printf("===== count %d\n", scopeMetrics.Len())
	for i := 0; i < scopeMetrics.Len(); i++ {
		metric := scopeMetrics.At(i)
		fmt.Printf("===== metric %v\n", metric.Name())
		if metric.Name() == "http_connected_total" {
			assert.Equal(m.t, float64(15), metric.Sum().DataPoints().At(0).DoubleValue())
			*m.httpConnected = true
		}
		if metric.Name() == "up" {
			assert.Equal(m.t, float64(1), metric.Gauge().DataPoints().At(0).DoubleValue())
			*m.up = true
		}
	}

	return nil
}

func TestNewPrometheusScraperBadInputs(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	leaderElection := LeaderElection{
		leading: true,
	}

	tests := []PrometheusScraperOpts{
		{
			Ctx:                 t.Context(),
			TelemetrySettings:   settings,
			Endpoint:            "",
			Consumer:            mockConsumer{},
			Host:                componenttest.NewNopHost(),
			ClusterNameProvider: mockClusterNameProvider{},
			LeaderElection:      nil,
		},
		{
			Ctx:                 t.Context(),
			TelemetrySettings:   settings,
			Endpoint:            "",
			Consumer:            nil,
			Host:                componenttest.NewNopHost(),
			ClusterNameProvider: mockClusterNameProvider{},
			LeaderElection:      &leaderElection,
		},
		{
			Ctx:                 t.Context(),
			TelemetrySettings:   settings,
			Endpoint:            "",
			Consumer:            mockConsumer{},
			Host:                nil,
			ClusterNameProvider: mockClusterNameProvider{},
			LeaderElection:      &leaderElection,
		},
		{
			Ctx:                 t.Context(),
			TelemetrySettings:   settings,
			Endpoint:            "",
			Consumer:            mockConsumer{},
			Host:                componenttest.NewNopHost(),
			ClusterNameProvider: nil,
			LeaderElection:      &leaderElection,
		},
	}

	for _, tt := range tests {
		scraper, err := NewPrometheusScraper(tt)

		assert.Error(t, err)
		assert.Nil(t, scraper)
	}
}

func TestNewPrometheusScraperEndToEnd(t *testing.T) {
	upPtr := false
	httpPtr := false

	mConsumer := mockConsumer{
		t:             t,
		up:            &upPtr,
		httpConnected: &httpPtr,
	}

	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	leaderElection := LeaderElection{
		leading: true,
	}

	// Create the scraper with an empty endpoint to avoid the StaticConfig issue.
	// The scraper creation will fail because it tries to create a prometheus receiver
	// with a StaticConfig that can't be marshaled to YAML. Instead, we'll create
	// the scraper struct manually and replace the prometheus receiver.
	scraper := &PrometheusScraper{
		ctx:                 t.Context(),
		settings:            settings,
		host:                componenttest.NewNopHost(),
		clusterNameProvider: mockClusterNameProvider{},
		leaderElection:      &leaderElection,
	}
	assert.Equal(t, mockClusterNameProvider{}, scraper.clusterNameProvider)

	// build up a new PR
	promFactory := prometheusreceiver.NewFactory()

	targets := []*mocks.TestData{
		{
			Name: "prometheus",
			Pages: []mocks.MockPrometheusResponse{
				{Code: 200, Data: renameMetric},
			},
		},
	}
	mp, cfg, err := mocks.SetupMockPrometheus(targets...)
	assert.NoError(t, err)

	// Use the config returned by SetupMockPrometheus directly, as it's already
	// properly loaded via promcfg.Load() with all discovery types registered.
	promConfig := prometheusreceiver.Config{
		PrometheusConfig: cfg,
	}

	// replace the prom receiver
	params := receiver.Settings{
		TelemetrySettings: scraper.GetSettings(),
		ID:                component.NewIDWithName(component.MustNewType("prometheus"), ""),
	}
	promReceiver, err := promFactory.CreateMetrics(scraper.GetContext(), params, &promConfig, mConsumer)
	assert.NoError(t, err)
	scraper.SetPrometheusReceiver(promReceiver)
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

	assert.True(t, *mConsumer.up)
	assert.True(t, *mConsumer.httpConnected)
}

func TestPrometheusScraperJobName(t *testing.T) {
	// needs to start with containerInsights
	assert.True(t, strings.HasPrefix(jobName, "containerInsightsKubeAPIServerScraper"))
}
