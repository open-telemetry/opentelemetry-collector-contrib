// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package raidscraper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/raidscraper/internal/metadata"
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
				BlocksTotal: 1,
			},
		}, nil
	}
	s.getMdraids = func() ([]Mdraid, error) {
		return []Mdraid{}, nil
	}

	require.NoError(t, s.start(ctx, componenttest.NewNopHost()))

	metrics, err := s.scrape(ctx)
	require.NoErrorf(t, err, "scrape error %+v", err)

	assert.Equal(t, 1, metrics.MetricCount())
	assert.Equal(t, 1, metrics.DataPointCount())
	metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())

	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, int64(1), dataPoint.IntValue())
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
	s.getMdraids = func() ([]Mdraid, error) {
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
	s.getMdraids = func() ([]Mdraid, error) {
		return []Mdraid{}, nil
	}

	require.NoError(t, s.start(ctx, componenttest.NewNopHost()))

	metrics, err := s.scrape(ctx)
	require.NoErrorf(t, err, "scrape error %+v", err)

	assert.Equal(t, 1, metrics.MetricCount())
	assert.Equal(t, 1, metrics.DataPointCount())

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
	s.getMdraids = func() ([]Mdraid, error) {
		return []Mdraid{}, nil
	}

	require.NoError(t, s.start(ctx, componenttest.NewNopHost()))

	metrics, err := s.scrape(ctx)
	require.NoErrorf(t, err, "scrape error %+v", err)

	assert.Equal(t, 0, metrics.MetricCount())
	assert.Equal(t, 0, metrics.DataPointCount())

}
