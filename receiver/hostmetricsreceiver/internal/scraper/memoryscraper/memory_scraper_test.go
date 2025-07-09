// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package memoryscraper

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/v4/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	type testCase struct {
		name                string
		virtualMemoryFunc   func(context.Context) (*mem.VirtualMemoryStat, error)
		expectedErr         string
		initializationErr   string
		config              *Config
		expectedMetricCount int
		bootTimeFunc        func(context.Context) (uint64, error)
	}

	testCases := []testCase{
		{
			name: "Standard",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedMetricCount: 1,
		},
		{
			name: "All metrics enabled",
			config: &Config{
				MetricsBuilderConfig: metadata.MetricsBuilderConfig{
					Metrics: metadata.MetricsConfig{
						SystemMemoryUtilization: metadata.MetricConfig{
							Enabled: true,
						},
						SystemMemoryUsage: metadata.MetricConfig{
							Enabled: true,
						},
						SystemMemoryPageSize: metadata.MetricConfig{
							Enabled: true,
						},
						SystemLinuxMemoryAvailable: metadata.MetricConfig{
							Enabled: true,
						},
						SystemLinuxMemoryDirty: metadata.MetricConfig{
							Enabled: true,
						},
					},
				},
			},
			expectedMetricCount: func() int {
				if runtime.GOOS == "linux" {
					return 5
				}
				return 3
			}(),
		},
		{
			name:              "Error",
			virtualMemoryFunc: func(context.Context) (*mem.VirtualMemoryStat, error) { return nil, errors.New("err1") },
			expectedErr:       "err1",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedMetricCount: 1,
		},
		{
			name:              "Error",
			bootTimeFunc:      func(context.Context) (uint64, error) { return 100, errors.New("err1") },
			initializationErr: "err1",
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedMetricCount: 1,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper := newMemoryScraper(context.Background(), scrapertest.NewNopSettings(metadata.Type), test.config)
			if test.virtualMemoryFunc != nil {
				scraper.virtualMemory = test.virtualMemoryFunc
			}
			if test.bootTimeFunc != nil {
				scraper.bootTime = test.bootTimeFunc
			}

			err := scraper.start(context.Background(), componenttest.NewNopHost())
			if test.initializationErr != "" {
				assert.EqualError(t, err, test.initializationErr)
				return
			}
			require.NoError(t, err, "Failed to initialize memory scraper: %v", err)
			md, err := scraper.scrape(context.Background())
			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)

				isPartial := scrapererror.IsPartialScrapeError(err)
				assert.True(t, isPartial)
				if isPartial {
					var scraperErr scrapererror.PartialScrapeError
					require.ErrorAs(t, err, &scraperErr)
					assert.Equal(t, metricsLen, scraperErr.Failed)
				}

				return
			}
			require.NoError(t, err, "Failed to scrape metrics: %v", err)

			assert.Equal(t, test.expectedMetricCount, md.MetricCount())

			metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			memUsageIdx := -1
			for i := 0; i < md.MetricCount(); i++ {
				if metrics.At(i).Name() == "system.memory.usage" {
					memUsageIdx = i
				}
			}
			assert.NotEqual(t, memUsageIdx, -1)
			assertMemoryUsageMetricValid(t, metrics.At(memUsageIdx), "system.memory.usage")

			if runtime.GOOS == "linux" {
				assertMemoryUsageMetricHasLinuxSpecificStateLabels(t, metrics.At(memUsageIdx))
			} else if runtime.GOOS != "windows" {
				internal.AssertSumMetricHasAttributeValue(t, metrics.At(memUsageIdx), 2, "state",
					pcommon.NewValueStr(metadata.AttributeStateInactive.String()))
			}

			internal.AssertSameTimeStampForAllMetrics(t, metrics)
		})
	}
}

func TestScrape_MemoryUtilization(t *testing.T) {
	type testCase struct {
		name              string
		virtualMemoryFunc func(context.Context) (*mem.VirtualMemoryStat, error)
		expectedErr       error
	}
	testCases := []testCase{
		{
			name: "Standard",
		},
		{
			name:              "Invalid total memory",
			virtualMemoryFunc: func(context.Context) (*mem.VirtualMemoryStat, error) { return &mem.VirtualMemoryStat{Total: 0}, nil },
			expectedErr:       ErrInvalidTotalMem,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			mbc := metadata.DefaultMetricsBuilderConfig()
			mbc.Metrics.SystemMemoryUtilization.Enabled = true
			mbc.Metrics.SystemMemoryUsage.Enabled = false
			scraperConfig := Config{
				MetricsBuilderConfig: mbc,
			}
			scraper := newMemoryScraper(context.Background(), scrapertest.NewNopSettings(metadata.Type), &scraperConfig)
			if test.virtualMemoryFunc != nil {
				scraper.virtualMemory = test.virtualMemoryFunc
			}

			err := scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize memory scraper: %v", err)

			md, err := scraper.scrape(context.Background())
			if test.expectedErr != nil {
				var partialScrapeErr scrapererror.PartialScrapeError
				assert.ErrorAs(t, err, &partialScrapeErr)
				return
			}
			require.NoError(t, err, "Failed to scrape metrics: %v", err)

			metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			assertMemoryUtilizationMetricValid(t, metrics.At(0), "system.memory.utilization")

			if runtime.GOOS == "linux" {
				assertMemoryUtilizationMetricHasLinuxSpecificStateLabels(t, metrics.At(0))
			} else if runtime.GOOS != "windows" {
				internal.AssertGaugeMetricHasAttributeValue(t, metrics.At(0), 2, "state",
					pcommon.NewValueStr(metadata.AttributeStateInactive.String()))
			}

			internal.AssertSameTimeStampForAllMetrics(t, metrics)
		})
	}
}

func assertMemoryUsageMetricValid(t *testing.T, metric pmetric.Metric, expectedName string) {
	assert.Equal(t, expectedName, metric.Name())
	assert.GreaterOrEqual(t, metric.Sum().DataPoints().Len(), 2)
	internal.AssertSumMetricHasAttributeValue(t, metric, 0, "state",
		pcommon.NewValueStr(metadata.AttributeStateUsed.String()))
	internal.AssertSumMetricHasAttributeValue(t, metric, 1, "state",
		pcommon.NewValueStr(metadata.AttributeStateFree.String()))
}

func assertMemoryUtilizationMetricValid(t *testing.T, metric pmetric.Metric, expectedName string) {
	assert.Equal(t, expectedName, metric.Name())
	assert.GreaterOrEqual(t, metric.Gauge().DataPoints().Len(), 2)
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 0, "state",
		pcommon.NewValueStr(metadata.AttributeStateUsed.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 1, "state",
		pcommon.NewValueStr(metadata.AttributeStateFree.String()))
}

func assertMemoryUsageMetricHasLinuxSpecificStateLabels(t *testing.T, metric pmetric.Metric) {
	internal.AssertSumMetricHasAttributeValue(t, metric, 2, "state",
		pcommon.NewValueStr(metadata.AttributeStateBuffered.String()))
	internal.AssertSumMetricHasAttributeValue(t, metric, 3, "state",
		pcommon.NewValueStr(metadata.AttributeStateCached.String()))
	internal.AssertSumMetricHasAttributeValue(t, metric, 4, "state",
		pcommon.NewValueStr(metadata.AttributeStateSlabReclaimable.String()))
	internal.AssertSumMetricHasAttributeValue(t, metric, 5, "state",
		pcommon.NewValueStr(metadata.AttributeStateSlabUnreclaimable.String()))
}

func assertMemoryUtilizationMetricHasLinuxSpecificStateLabels(t *testing.T, metric pmetric.Metric) {
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 2, "state",
		pcommon.NewValueStr(metadata.AttributeStateBuffered.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 3, "state",
		pcommon.NewValueStr(metadata.AttributeStateCached.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 4, "state",
		pcommon.NewValueStr(metadata.AttributeStateSlabReclaimable.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 5, "state",
		pcommon.NewValueStr(metadata.AttributeStateSlabUnreclaimable.String()))
}
