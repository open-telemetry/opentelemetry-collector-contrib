// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package loadscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/load"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters"
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
)

var (
	scraperCount int
	startupLock  sync.Mutex

	samplerInstance *sampler
)

type sampler struct {
	done               chan struct{}
	logger             *zap.Logger
	perfCounterScraper perfcounters.PerfCounterScraper
	loadAvg1m          float64
	loadAvg5m          float64
	loadAvg15m         float64
	lock               sync.RWMutex
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
		return err
	}

	samplerInstance.startSamplingTicker()
	return nil
}

func newSampler(logger *zap.Logger) (*sampler, error) {
	perfCounterScraper := &perfcounters.PerfLibScraper{}

	if err := perfCounterScraper.Initialize(system); err != nil {
		return nil, err
	}

	sampler := &sampler{
		logger:             logger,
		perfCounterScraper: perfCounterScraper,
		done:               make(chan struct{}),
	}

	return sampler, nil
}

func (sw *sampler) startSamplingTicker() {
	go func() {
		ticker := time.NewTicker(samplingFrequency)
		defer ticker.Stop()

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
	counters, err := sw.perfCounterScraper.Scrape()
	if err != nil {
		sw.logger.Error("Load Scraper: failed to measure processor queue length", zap.Error(err))
		return
	}

	systemObject, err := counters.GetObject(system)
	if err != nil {
		sw.logger.Error("Load Scraper: failed to measure processor queue length", zap.Error(err))
		return
	}

	counterValues, err := systemObject.GetValues(processorQueueLength)
	if err != nil {
		sw.logger.Error("Load Scraper: failed to measure processor queue length", zap.Error(err))
		return
	}

	currentLoad := float64(counterValues[0].Values[processorQueueLength])

	sw.lock.Lock()
	defer sw.lock.Unlock()
	sw.loadAvg1m = sw.loadAvg1m*loadAvgFactor1m + currentLoad*(1-loadAvgFactor1m)
	sw.loadAvg5m = sw.loadAvg5m*loadAvgFactor5m + currentLoad*(1-loadAvgFactor5m)
	sw.loadAvg15m = sw.loadAvg15m*loadAvgFactor15m + currentLoad*(1-loadAvgFactor15m)
}

func stopSampling(_ context.Context) error {
	startupLock.Lock()
	defer startupLock.Unlock()

	// only stop sampling if all load scrapers have been closed
	scraperCount--
	if scraperCount > 0 {
		return nil
	}

	close(samplerInstance.done)
	return nil
}

func getSampledLoadAverages() (*load.AvgStat, error) {
	samplerInstance.lock.RLock()
	defer samplerInstance.lock.RUnlock()

	avgStat := &load.AvgStat{
		Load1:  samplerInstance.loadAvg1m,
		Load5:  samplerInstance.loadAvg5m,
		Load15: samplerInstance.loadAvg15m,
	}

	return avgStat, nil
}
