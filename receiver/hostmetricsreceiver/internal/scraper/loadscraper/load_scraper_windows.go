// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package loadscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/load"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
)

// Sample processor queue length at a 5s frequency, and calculate exponentially weighted moving averages
// as per https://en.wikipedia.org/wiki/Load_(computing)#Unix-style_load_calculation

const (
	system               = "System"
	processorQueueLength = "Processor Queue Length"
)

var (
	samplingFrequency = 5 * time.Second

	loadAvgFactor1m  = 1 / math.Exp(samplingFrequency.Seconds()/time.Minute.Seconds())
	loadAvgFactor5m  = 1 / math.Exp(samplingFrequency.Seconds()/(5*time.Minute).Seconds())
	loadAvgFactor15m = 1 / math.Exp(samplingFrequency.Seconds()/(15*time.Minute).Seconds())

	// perfCounterFactory is used to facilitate testing
	perfCounterFactory = winperfcounters.NewWatcher
)

var (
	scraperCount int
	startupLock  sync.Mutex

	samplerInstance *sampler
)

type sampler struct {
	done               chan struct{}
	logger             *zap.Logger
	perfCounterWatcher winperfcounters.PerfCounterWatcher
	loadAvg1m          float64
	loadAvg5m          float64
	loadAvg15m         float64
	lock               sync.RWMutex
}

func setSamplingFrequency(freq time.Duration) {
	samplingFrequency = freq
}

func startSampling(_ context.Context, logger *zap.Logger) error {
	startupLock.Lock()
	defer startupLock.Unlock()

	// startSampling may be called multiple times if multiple scrapers are
	// initialized - but we only want to initialize a single load sampler
	scraperCount++
	if scraperCount > 1 {
		return nil
	}

	var err error
	samplerInstance, err = newSampler(logger)
	if err != nil {
		// To keep the same behavior, as previous versions on Windows, error in this case is just logged
		// and the scraper is not started.
		scraperCount = 0
		logger.Error("Failed to init performance counters, load metrics will not be scraped", zap.Error(err))
		return errPreventScrape
	}

	samplerInstance.startSamplingTicker()
	return nil
}

func newSampler(logger *zap.Logger) (*sampler, error) {
	perfCounterWatcher, err := perfCounterFactory(system, "", processorQueueLength)
	if err != nil {
		return nil, err
	}

	sampler := &sampler{
		logger:             logger,
		perfCounterWatcher: perfCounterWatcher,
		done:               make(chan struct{}),
	}

	return sampler, nil
}

func (sw *sampler) startSamplingTicker() {
	// Store the sampling frequency in a local variable to avoid race conditions during tests.
	frequency := samplingFrequency
	go func() {
		ticker := time.NewTicker(frequency)
		defer ticker.Stop()

		sw.sampleLoad()
		for {
			select {
			case <-ticker.C:
				sw.sampleLoad()
			case <-sw.done:
				return
			}
		}
	}()
}

func (sw *sampler) sampleLoad() {
	var currentLoadRaw int64
	ok, err := sw.perfCounterWatcher.ScrapeRawValue(&currentLoadRaw)
	if err != nil {
		sw.logger.Error("Load Scraper: failed to measure processor queue length", zap.Error(err))
		return
	}

	if !ok {
		sw.logger.Error("Load Scraper: failed to measure processor queue length, no data returned")
		return
	}

	currentLoad := float64(currentLoadRaw)

	sw.lock.Lock()
	defer sw.lock.Unlock()
	sw.loadAvg1m = sw.loadAvg1m*loadAvgFactor1m + currentLoad*(1-loadAvgFactor1m)
	sw.loadAvg5m = sw.loadAvg5m*loadAvgFactor5m + currentLoad*(1-loadAvgFactor5m)
	sw.loadAvg15m = sw.loadAvg15m*loadAvgFactor15m + currentLoad*(1-loadAvgFactor15m)
}

func stopSampling(_ context.Context) error {
	startupLock.Lock()
	defer startupLock.Unlock()

	if scraperCount == 0 {
		// no load scraper is running nothing to do
		return nil
	}
	// only stop sampling if all load scrapers have been closed
	scraperCount--
	if scraperCount > 0 {
		return nil
	}

	// no more load scrapers are running, stop the sampler
	close(samplerInstance.done)
	return nil
}

func getSampledLoadAverages(_ context.Context) (*load.AvgStat, error) {
	samplerInstance.lock.RLock()
	defer samplerInstance.lock.RUnlock()

	avgStat := &load.AvgStat{
		Load1:  samplerInstance.loadAvg1m,
		Load5:  samplerInstance.loadAvg5m,
		Load15: samplerInstance.loadAvg15m,
	}

	return avgStat, nil
}
