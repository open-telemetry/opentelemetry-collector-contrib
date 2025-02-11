// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper"

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
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

type MetricLabel struct {
	LabelName  string
	LabelValue string
}

type ExpectedMetricStruct struct {
	MetricValue  float64
	MetricLabels []MetricLabel
}

type TestSimplePrometheusEndToEndOpts struct {
	T                   *testing.T
	Consumer            consumer.Metrics
	DataReturned        string
	ScraperOpts         SimplePrometheusScraperOpts
	MetricRelabelConfig []*relabel.Config
}

type MockConsumer struct {
	T               *testing.T
	ExpectedMetrics map[string]ExpectedMetricStruct
}

func (m MockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m MockConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	expectedMetricsCount := len(m.ExpectedMetrics)
	metricFoundCount := 0

	assert.Equal(m.T, 1, md.ResourceMetrics().Len())

	scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < scopeMetrics.Len(); i++ {
		metric := scopeMetrics.At(i)
		metricsStruct, ok := m.ExpectedMetrics[metric.Name()]
		if ok {
			assert.Equal(m.T, metricsStruct.MetricValue, metric.Gauge().DataPoints().At(0).DoubleValue())
			for _, expectedLabel := range metricsStruct.MetricLabels {
				labelValue, isFound := metric.Gauge().DataPoints().At(0).Attributes().Get(expectedLabel.LabelName)
				assert.True(m.T, isFound)
				assert.Equal(m.T, expectedLabel.LabelValue, labelValue.Str())
			}
			metricFoundCount++
		}
	}

	assert.Equal(m.T, expectedMetricsCount, metricFoundCount)

	return nil
}

func TestSimplePrometheusEndToEnd(opts TestSimplePrometheusEndToEndOpts) {
	scraper, err := NewSimplePrometheusScraper(opts.ScraperOpts)
	assert.NoError(opts.T, err)

	// build up a new PR
	promFactory := prometheusreceiver.NewFactory()

	targets := []*mocks.TestData{
		{
			Name: "neuron",
			Pages: []mocks.MockPrometheusResponse{
				{Code: 200, Data: opts.DataReturned},
			},
		},
	}
	mp, cfg, err := mocks.SetupMockPrometheus(targets...)
	assert.NoError(opts.T, err)

	split := strings.Split(mp.Srv.URL, "http://")

	mockedScrapeConfig := &config.ScrapeConfig{
		ScrapeProtocols: config.DefaultScrapeProtocols,
		HTTPClientConfig: configutil.HTTPClientConfig{
			TLSConfig: configutil.TLSConfig{
				InsecureSkipVerify: true,
			},
		},
		ScrapeInterval:  cfg.ScrapeConfigs[0].ScrapeInterval,
		ScrapeTimeout:   cfg.ScrapeConfigs[0].ScrapeInterval,
		JobName:         fmt.Sprintf("%s/%s", "jobName", cfg.ScrapeConfigs[0].MetricsPath),
		HonorTimestamps: true,
		Scheme:          "http",
		MetricsPath:     cfg.ScrapeConfigs[0].MetricsPath,
		ServiceDiscoveryConfigs: discovery.Configs{
			// using dummy static config to avoid service discovery initialization
			&discovery.StaticConfig{
				{
					Targets: []model.LabelSet{
						{
							model.AddressLabel: model.LabelValue(split[1]),
						},
					},
				},
			},
		},
		RelabelConfigs:       []*relabel.Config{},
		MetricRelabelConfigs: opts.MetricRelabelConfig,
	}

	promConfig := prometheusreceiver.Config{
		PrometheusConfig: &prometheusreceiver.PromConfig{
			ScrapeConfigs: []*config.ScrapeConfig{mockedScrapeConfig},
		},
	}

	// replace the prom receiver
	params := receiver.Settings{
		TelemetrySettings: scraper.Settings,
	}
	scraper.PrometheusReceiver, err = promFactory.CreateMetrics(scraper.Ctx, params, &promConfig, opts.Consumer)
	assert.NoError(opts.T, err)
	assert.NotNil(opts.T, mp)
	defer mp.Close()

	// perform a single scrape, this will kick off the scraper process for additional scrapes
	scraper.GetMetrics()

	opts.T.Cleanup(func() {
		scraper.Shutdown()
	})

	// wait for 2 scrapes, one initiated by us, another by the new scraper process
	mp.Wg.Wait()
	mp.Wg.Wait()
}
