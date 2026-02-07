// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package memoryscraper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper/internal/metadata"
)

func TestScrape_UseMemAvailable(t *testing.T) {
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics.SystemMemoryUtilization.Enabled = true
	mbc.Metrics.SystemMemoryUsage.Enabled = true
	scraperConfig := Config{
		MetricsBuilderConfig: mbc,
	}
	scraper := newMemoryScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &scraperConfig)

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err, "Failed to initialize memory scraper: %v", err)

	memInfo, err := scraper.virtualMemory(t.Context())
	require.NoError(t, err)
	require.NotNil(t, memInfo)

	scraper.recordMemoryUsageMetric(pcommon.NewTimestampFromTime(time.Now()), memInfo)
	scraper.recordMemoryUtilizationMetric(pcommon.NewTimestampFromTime(time.Now()), memInfo)
	memUsedMd := scraper.mb.Emit()

	// disable feature gate
	_ = featuregate.GlobalRegistry().Set(
		"receiver.hostmetricsreceiver.UseLinuxMemAvailable", false)
	t.Cleanup(func() {
		_ = featuregate.GlobalRegistry().Set("receiver.hostmetricsreceiver.UseLinuxMemAvailable", true)
	})
	scraper.recordMemoryUsageMetric(pcommon.NewTimestampFromTime(time.Now()), memInfo)
	scraper.recordMemoryUtilizationMetric(pcommon.NewTimestampFromTime(time.Now()), memInfo)
	legacyMd := scraper.mb.Emit()

	// Used memory calculation based on MemAvailable is greater than "Total
	// - Free - Buffers - Cache" as it takes into account the amount of
	// Cached memory that is not freeable.
	assert.Greater(t, memUsedMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).IntValue(), legacyMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).IntValue(), "system.memory.usage for the used state should be greater when computed using memAvailable")
	assert.Greater(t, memUsedMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().At(0).DoubleValue(), legacyMd.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1).Gauge().DataPoints().At(0).DoubleValue(), "system.memory.utilization for the used state should be greater when computed using memAvailable")
}

func TestScrape_SharedMemory(t *testing.T) {
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics.SystemMemoryLinuxShared.Enabled = true
	scraperConfig := Config{
		MetricsBuilderConfig: mbc,
	}
	scraper := newMemoryScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &scraperConfig)

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err, "Failed to initialize memory scraper: %v", err)

	memInfo, err := scraper.virtualMemory(t.Context())
	require.NoError(t, err)
	require.NotNil(t, memInfo)

	scraper.recordMemoryLinuxSharedMetric(pcommon.NewTimestampFromTime(time.Now()), memInfo)
	md := scraper.mb.Emit()

	require.Equal(t, 1, md.ResourceMetrics().Len())
	require.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().Len())

	metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	require.Equal(t, 1, metrics.Len(), "Expected exactly one metric")

	metric := metrics.At(0)
	assert.Equal(t, "system.memory.linux.shared", metric.Name())
	assert.Equal(t, pmetric.MetricTypeSum, metric.Type())

	dataPoints := metric.Sum().DataPoints()
	require.Equal(t, 1, dataPoints.Len(), "Expected exactly one data point")

	assert.GreaterOrEqual(t, dataPoints.At(0).IntValue(), int64(0))

	_, hasState := dataPoints.At(0).Attributes().Get("state")
	assert.False(t, hasState, "system.memory.shared should not have a state attribute")
}
