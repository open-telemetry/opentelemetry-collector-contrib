// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper"

import (
	"context"
	"slices"
	"testing"

	_ "github.com/prometheus/prometheus/discovery/install" // init() registers service discovery impl.
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
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
	T                *testing.T
	ExpectedMetrics  map[string]ExpectedMetricStruct
	AdditionalLabels []string
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
			for _, specialLabel := range m.AdditionalLabels {
				isLabelInExpected := slices.ContainsFunc(metricsStruct.MetricLabels, func(label MetricLabel) bool { return label.LabelName == specialLabel })
				_, isLabelInActual := metric.Gauge().DataPoints().At(0).Attributes().Get(specialLabel)
				assert.Equal(m.T, isLabelInExpected, isLabelInActual)
			}
			metricFoundCount++
		}
	}

	assert.Equal(m.T, expectedMetricsCount, metricFoundCount)

	return nil
}

func TestSimplePrometheusEndToEnd(opts TestSimplePrometheusEndToEndOpts) {
	// Create the scraper struct directly without calling the prometheus factory
	// with production configs that use kubernetes SD (which can't be marshaled to YAML).
	// We'll replace the PrometheusReceiver with one created using the mock config.
	scraper := &SimplePrometheusScraper{
		Ctx:              opts.ScraperOpts.Ctx,
		Settings:         opts.ScraperOpts.TelemetrySettings,
		Host:             opts.ScraperOpts.Host,
		HostInfoProvider: opts.ScraperOpts.HostInfoProvider,
		ScraperConfigs:   opts.ScraperOpts.ScraperConfigs,
	}

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

	// Apply metric relabel configs if provided
	if len(opts.MetricRelabelConfig) > 0 && len(cfg.ScrapeConfigs) > 0 {
		cfg.ScrapeConfigs[0].MetricRelabelConfigs = opts.MetricRelabelConfig
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
