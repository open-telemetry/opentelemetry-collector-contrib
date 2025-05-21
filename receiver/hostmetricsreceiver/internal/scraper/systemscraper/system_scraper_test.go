// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemscraper

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/systemscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	if !supportedOS {
		t.Skip()
	}

	ctx := context.Background()
	fakeDate := time.Date(2006, 0o1, 0o2, 0o3, 0o4, 0o5, 0, time.UTC)

	s := newSystemScraper(ctx, scrapertest.NewNopSettings(metadata.Type), &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	})

	// mock
	s.bootTime = func(_ context.Context) (uint64, error) {
		return uint64(fakeDate.Unix()), nil
	}
	s.uptime = func(_ context.Context) (uint64, error) {
		return uint64(123456), nil
	}

	require.NoError(t, s.start(ctx, componenttest.NewNopHost()))

	metrics, err := s.scrape(ctx)
	require.NoErrorf(t, err, "scrape error %+v", err)

	assert.Equal(t, 1, metrics.MetricCount())
	assert.Equal(t, 1, metrics.DataPointCount())

	metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equalf(t, pmetric.MetricTypeGauge, metric.Type(), "invalid metric type: %v", metric.Type())

	dataPoint := metric.Gauge().DataPoints().At(0)
	assert.Equal(t, float64(123456), dataPoint.DoubleValue())
	assert.Equal(t, fakeDate, dataPoint.StartTimestamp().AsTime())
}
