// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadscraper

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/load"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper/internal/metadata"
)

const (
	testStandard = "Standard"
	testAverage  = "PerCPUEnabled"
	bootTime     = 100
)

func TestScrape(t *testing.T) {
	type testCase struct {
		name         string
		bootTimeFunc func(context.Context) (uint64, error)
		loadFunc     func(context.Context) (*load.AvgStat, error)
		expectedErr  string
		saveMetrics  bool
		config       *Config
	}

	testCases := []testCase{
		{
			name:        testStandard,
			saveMetrics: true,
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
		},
		{
			name:        testAverage,
			saveMetrics: true,
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				CPUAverage:           true,
			},
			bootTimeFunc: func(context.Context) (uint64, error) { return bootTime, nil },
		},
		{
			name:     "Load Error",
			loadFunc: func(context.Context) (*load.AvgStat, error) { return nil, errors.New("err1") },
			config: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedErr: "err1",
		},
	}
	results := make(map[string]pmetric.MetricSlice)
	// triggers scraping to start
	startChan := make(chan bool)
	// used to lock results map in order to avoid concurrent map writes
	resultsMapLock := sync.Mutex{}

	testFn := func(t *testing.T, test testCase) {
		// wait for messurement to start
		<-startChan

		scraper := newLoadScraper(context.Background(), receivertest.NewNopSettings(), test.config)
		if test.loadFunc != nil {
			scraper.load = test.loadFunc
		}
		if test.bootTimeFunc != nil {
			scraper.bootTime = test.bootTimeFunc
		}

		err := scraper.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err, "Failed to initialize load scraper: %v", err)
		defer func() { assert.NoError(t, scraper.shutdown(context.Background())) }()
		if runtime.GOOS == "windows" {
			// let it sample
			<-time.After(3 * time.Second)
		}

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

		if test.bootTimeFunc != nil {
			actualBootTime, err := scraper.bootTime(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, uint64(bootTime), actualBootTime)
		}
		// expect 3 metrics
		assert.Equal(t, 3, md.MetricCount())

		metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
		// expect a single datapoint for 1m, 5m & 15m load metrics
		assertMetricHasSingleDatapoint(t, metrics.At(0), "system.cpu.load_average.15m")
		assertMetricHasSingleDatapoint(t, metrics.At(1), "system.cpu.load_average.1m")
		assertMetricHasSingleDatapoint(t, metrics.At(2), "system.cpu.load_average.5m")

		internal.AssertSameTimeStampForAllMetrics(t, metrics)

		// save metrics for additional tests if flag is enabled
		if test.saveMetrics {
			resultsMapLock.Lock()
			results[test.name] = metrics
			resultsMapLock.Unlock()
		}
	}

	// used to wait for each test to start to make sure they are all sampling at the same time
	var startWg sync.WaitGroup
	startWg.Add(len(testCases))

	// used to wait for each test to finish
	var waitWg sync.WaitGroup
	waitWg.Add(len(testCases))

	setSamplingFrequency(500 * time.Millisecond)

	for _, test := range testCases {
		go func(t *testing.T, test testCase) {
			startWg.Done()
			testFn(t, test)
			waitWg.Done()
		}(t, test)
	}

	// wait for test goroutines to start
	startWg.Wait()
	// trigger tests
	close(startChan)
	// wait for tests to finish
	waitWg.Wait()

	// Additional test for average per CPU
	numCPU := runtime.NumCPU()
	for i := 0; i < results[testStandard].Len(); i++ {
		assertCompareAveragePerCPU(t, results[testAverage].At(i), results[testStandard].At(i), numCPU)
	}
}

func assertMetricHasSingleDatapoint(t *testing.T, metric pmetric.Metric, expectedName string) {
	assert.Equal(t, expectedName, metric.Name())
	assert.Equal(t, 1, metric.Gauge().DataPoints().Len())
}

func assertCompareAveragePerCPU(t *testing.T, average pmetric.Metric, standard pmetric.Metric, numCPU int) {
	valAverage := average.Gauge().DataPoints().At(0).DoubleValue()
	valStandard := standard.Gauge().DataPoints().At(0).DoubleValue()
	if valAverage == 0 && valStandard == 0 {
		// nothing to compare, queue is empty
		return
	}
	if numCPU == 1 {
		// For hardware with only 1 cpu, results must be very close
		assert.InDelta(t, valAverage, valStandard, 0.1)
	} else {
		// For hardward with multiple CPU, average per cpu is fatally less than standard
		assert.Less(t, valAverage, valStandard)
	}
}
