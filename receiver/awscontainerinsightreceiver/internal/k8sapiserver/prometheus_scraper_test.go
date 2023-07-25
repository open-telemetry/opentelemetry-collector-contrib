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

package k8sapiserver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

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
	t                *testing.T
	up               *bool
	httpConnected    *bool
	rpcDurationTotal *bool
}

func (m mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m mockConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	assert.Equal(m.t, 1, md.ResourceMetrics().Len())

	scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < scopeMetrics.Len(); i++ {
		metric := scopeMetrics.At(i)
		if metric.Name() == "http_connected_total" {
			assert.Equal(m.t, float64(15), metric.Sum().DataPoints().At(0).DoubleValue())
			*m.httpConnected = true
		}
		if metric.Name() == "rpc_duration_total" {
			*m.rpcDurationTotal = true
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
			Ctx:                 context.TODO(),
			TelemetrySettings:   settings,
			Endpoint:            "",
			Consumer:            mockConsumer{},
			Host:                componenttest.NewNopHost(),
			ClusterNameProvider: mockClusterNameProvider{},
			LeaderElection:      nil,
			BearerToken:         "",
		},
		{
			Ctx:                 context.TODO(),
			TelemetrySettings:   settings,
			Endpoint:            "",
			Consumer:            nil,
			Host:                componenttest.NewNopHost(),
			ClusterNameProvider: mockClusterNameProvider{},
			LeaderElection:      &leaderElection,
			BearerToken:         "",
		},
		{
			Ctx:                 context.TODO(),
			TelemetrySettings:   settings,
			Endpoint:            "",
			Consumer:            mockConsumer{},
			Host:                nil,
			ClusterNameProvider: mockClusterNameProvider{},
			LeaderElection:      &leaderElection,
			BearerToken:         "",
		},
		{
			Ctx:                 context.TODO(),
			TelemetrySettings:   settings,
			Endpoint:            "",
			Consumer:            mockConsumer{},
			Host:                componenttest.NewNopHost(),
			ClusterNameProvider: nil,
			LeaderElection:      &leaderElection,
			BearerToken:         "",
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
	rpcDurationTotalPtr := false

	consumer := mockConsumer{
		t:                t,
		up:               &upPtr,
		httpConnected:    &httpPtr,
		rpcDurationTotal: &rpcDurationTotalPtr,
	}

	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	leaderElection := LeaderElection{
		leading: true,
	}

	scraper, err := NewPrometheusScraper(PrometheusScraperOpts{
		Ctx:                 context.TODO(),
		TelemetrySettings:   settings,
		Endpoint:            "",
		Consumer:            mockConsumer{},
		Host:                componenttest.NewNopHost(),
		ClusterNameProvider: mockClusterNameProvider{},
		LeaderElection:      &leaderElection,
		BearerToken:         "",
	})
	assert.NoError(t, err)
	assert.Equal(t, mockClusterNameProvider{}, scraper.clusterNameProvider)

	// build up a new PR
	promFactory := prometheusreceiver.NewFactory()

	targets := []*testData{
		{
			name: "prometheus",
			pages: []mockPrometheusResponse{
				{code: 200, data: renameMetric},
			},
		},
	}
	mp, cfg, err := setupMockPrometheus(targets...)
	assert.NoError(t, err)

	split := strings.Split(mp.srv.URL, "http://")

	scrapeConfig := &config.ScrapeConfig{
		HTTPClientConfig: configutil.HTTPClientConfig{
			TLSConfig: configutil.TLSConfig{
				InsecureSkipVerify: true,
			},
		},
		ScrapeInterval:  cfg.ScrapeConfigs[0].ScrapeInterval,
		ScrapeTimeout:   cfg.ScrapeConfigs[0].ScrapeInterval,
		JobName:         fmt.Sprintf("%s/%s", "containerInsightsKubeAPIServerScraper", cfg.ScrapeConfigs[0].MetricsPath),
		HonorTimestamps: true,
		Scheme:          "http",
		MetricsPath:     cfg.ScrapeConfigs[0].MetricsPath,
		ServiceDiscoveryConfigs: discovery.Configs{
			&discovery.StaticConfig{
				{
					Targets: []model.LabelSet{
						{
							model.AddressLabel: model.LabelValue(split[1]),
							"ClusterName":      model.LabelValue("test_cluster_name"),
							"Version":          model.LabelValue("0"),
							"Sources":          model.LabelValue("[\"apiserver\"]"),
							"NodeName":         model.LabelValue("test"),
							"Type":             model.LabelValue("control_plane"),
						},
					},
				},
			},
		},
		MetricRelabelConfigs: []*relabel.Config{
			{
				// allow list filter for the control plane metrics we care about
				SourceLabels: model.LabelNames{"__name__"},
				Regex:        relabel.MustNewRegexp("http_connected_total"),
				Action:       relabel.Keep,
			},
		},
	}

	promConfig := prometheusreceiver.Config{
		PrometheusConfig: &config.Config{
			ScrapeConfigs: []*config.ScrapeConfig{scrapeConfig},
		},
	}

	// replace the prom receiver
	params := receiver.CreateSettings{
		TelemetrySettings: scraper.settings,
	}
	scraper.prometheusReceiver, err = promFactory.CreateMetricsReceiver(scraper.ctx, params, &promConfig, consumer)
	assert.NoError(t, err)
	assert.NotNil(t, mp)
	defer mp.Close()

	// perform a single scrape, this will kick off the scraper process for additional scrapes
	scraper.GetMetrics()

	t.Cleanup(func() {
		scraper.Shutdown()
	})

	// wait for 2 scrapes, one initiated by us, another by the new scraper process
	mp.wg.Wait()
	mp.wg.Wait()

	assert.True(t, *consumer.up)
	assert.True(t, *consumer.httpConnected)
	assert.False(t, *consumer.rpcDurationTotal) // this will get filtered out by our metric relabel config
}
