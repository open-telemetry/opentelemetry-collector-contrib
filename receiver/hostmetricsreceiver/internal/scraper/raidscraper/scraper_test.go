// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package raidscraper

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/raidscraper/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapertest"
)

func TestScrapeSimple(t *testing.T) {
	ctx := context.Background()

	s, _ := newRaidScraper(ctx, scrapertest.NewNopSettings(metadata.Type), &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	})

	// mock
	s.getMdStats = func() ([]MDStat, error) {
		return []MDStat{
			{
				Name:          "disk1",
				BlocksTotal:   2,
				BlocksSynced:  1,
				ActivityState: "resyncing",
				DisksActive:   5,
				DisksFailed:   2,
				DisksSpare:    3,
			},
		}, nil
	}
	s.getMdraids = func(_ string) ([]Mdraid, error) {
		return []Mdraid{
			{
				Device:        "disk1",
				DegradedDisks: 2,
				Disks:         4,
			},
		}, nil
	}

	require.NoError(t, s.start(ctx, componenttest.NewNopHost()))

	metrics, err := s.scrape(ctx)
	require.NoErrorf(t, err, "scrape error %+v", err)

	assert.Equal(t, 7, metrics.MetricCount())
	assert.Equal(t, 13, metrics.DataPointCount())
	metric := getMetric(t, "system.linux.mdraid.blocks.total", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp := getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(2), dp.IntValue())

	metric = getMetric(t, "system.linux.mdraid.blocks.synced", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp = getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(1), dp.IntValue())

	metric = getMetric(t, "system.linux.mdraid.disks", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp = getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1", "state": "spare"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(3), dp.IntValue())

	metric = getMetric(t, "system.linux.mdraid.disks", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp = getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1", "state": "active"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(5), dp.IntValue())

	metric = getMetric(t, "system.linux.mdraid.disks", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp = getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1", "state": "failed"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(2), dp.IntValue())

	metric = getMetric(t, "system.linux.mdraid.activity_state", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp = getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1", "activity_state": "resync"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(1), dp.IntValue())

	metric = getMetric(t, "system.linux.mdraid.activity_state", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp = getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1", "activity_state": "active"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(0), dp.IntValue())

	metric = getMetric(t, "system.linux.mdraid.activity_state", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp = getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1", "activity_state": "recovering"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(0), dp.IntValue())

	metric = getMetric(t, "system.linux.mdraid.activity_state", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp = getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1", "activity_state": "inactive"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(0), dp.IntValue())

	metric = getMetric(t, "system.linux.mdraid.activity_state", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp = getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1", "activity_state": "check"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(0), dp.IntValue())

	metric = getMetric(t, "system.linux.mdraid.raid_disks", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp = getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(4), dp.IntValue())

	metric = getMetric(t, "system.linux.mdraid.degraded_raid_disks", metrics.ResourceMetrics())
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())
	dp = getGaugeDatapointIntValueByLabels(metric.Gauge(), map[string]string{"device": "disk1"})
	assert.NotNil(t, dp)
	assert.Equal(t, int64(2), dp.IntValue())
}

func TestScrapeExclude(t *testing.T) {
	ctx := context.Background()

	s, _ := newRaidScraper(ctx, scrapertest.NewNopSettings(metadata.Type), &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Exclude: MatchConfig{
			Devices: []string{"test"},
			Config:  filterset.Config{MatchType: filterset.Strict},
		},
	})

	// mock
	s.getMdStats = func() ([]MDStat, error) {
		return []MDStat{
			{
				Name:        "test",
				BlocksTotal: 1,
			},
		}, nil
	}
	s.getMdraids = func(_ string) ([]Mdraid, error) {
		return []Mdraid{}, nil
	}

	require.NoError(t, s.start(ctx, componenttest.NewNopHost()))

	metrics, err := s.scrape(ctx)
	require.NoErrorf(t, err, "scrape error %+v", err)

	assert.Equal(t, 0, metrics.MetricCount())
	assert.Equal(t, 0, metrics.DataPointCount())
}

func TestScrapeInclude(t *testing.T) {
	ctx := context.Background()

	s, _ := newRaidScraper(ctx, scrapertest.NewNopSettings(metadata.Type), &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Include: MatchConfig{
			Devices: []string{"test"},
			Config:  filterset.Config{MatchType: filterset.Strict},
		},
	})

	// mock
	s.getMdStats = func() ([]MDStat, error) {
		return []MDStat{
			{
				Name:        "test",
				BlocksTotal: 1,
			},
		}, nil
	}
	s.getMdraids = func(_ string) ([]Mdraid, error) {
		return []Mdraid{}, nil
	}

	require.NoError(t, s.start(ctx, componenttest.NewNopHost()))

	metrics, err := s.scrape(ctx)
	require.NoErrorf(t, err, "scrape error %+v", err)

	assert.Equal(t, 5, metrics.MetricCount())
	assert.Equal(t, 11, metrics.DataPointCount())
}

func TestScrapeExcludeOverridesInclude(t *testing.T) {
	ctx := context.Background()

	s, _ := newRaidScraper(ctx, scrapertest.NewNopSettings(metadata.Type), &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Include: MatchConfig{
			Devices: []string{"test"},
			Config:  filterset.Config{MatchType: filterset.Strict},
		},
		Exclude: MatchConfig{
			Devices: []string{"test"},
			Config:  filterset.Config{MatchType: filterset.Strict},
		},
	})

	// mock
	s.getMdStats = func() ([]MDStat, error) {
		return []MDStat{
			{
				Name:        "test",
				BlocksTotal: 1,
			},
		}, nil
	}
	s.getMdraids = func(_ string) ([]Mdraid, error) {
		return []Mdraid{}, nil
	}

	require.NoError(t, s.start(ctx, componenttest.NewNopHost()))

	metrics, err := s.scrape(ctx)
	require.NoErrorf(t, err, "scrape error %+v", err)

	assert.Equal(t, 0, metrics.MetricCount())
	assert.Equal(t, 0, metrics.DataPointCount())
}

func getMetric(t *testing.T, expectedMetricName string, rms pmetric.ResourceMetricsSlice) pmetric.Metric {
	for i := 0; i < rms.Len(); i++ {
		metrics := getMetricSlice(t, rms.At(i))
		for j := 0; j < metrics.Len(); j++ {
			metric := metrics.At(j)
			if metric.Name() == expectedMetricName {
				return metric
			}
		}
	}

	require.Fail(t, fmt.Sprintf("no metric with name %s was returned", expectedMetricName))
	return pmetric.NewMetric()
}

func getGaugeDatapointIntValueByLabels(metric pmetric.Gauge, labels map[string]string) *pmetric.NumberDataPoint {
	for _, dp := range metric.DataPoints().All() {
		matches := len(labels)
		for label, labelValue := range labels {
			if value, ok := dp.Attributes().Get(label); ok {
				if value.Str() == labelValue {
					matches--
				}
			}
			if matches == 0 {
				return &dp
			}
		}
	}
	return nil
}

func getMetricSlice(t *testing.T, rm pmetric.ResourceMetrics) pmetric.MetricSlice {
	ilms := rm.ScopeMetrics()
	require.Equal(t, 1, ilms.Len())
	return ilms.At(0).Metrics()
}
