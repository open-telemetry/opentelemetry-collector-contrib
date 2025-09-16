// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/collectors"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/rpc"
)

// Registry manages all available collectors
type Registry struct {
	collectors map[string]Collector
	mu         sync.RWMutex
}

// NewRegistry creates a new collector registry
func NewRegistry() *Registry {
	return &Registry{
		collectors: make(map[string]Collector),
	}
}

// Register adds a collector to the registry
func (r *Registry) Register(collector Collector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collectors[collector.Name()] = collector
}

// GetCollector returns a collector by name
func (r *Registry) GetCollector(name string) (Collector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	collector, exists := r.collectors[name]
	return collector, exists
}

// GetAllCollectors returns all registered collectors
func (r *Registry) GetAllCollectors() map[string]Collector {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]Collector)
	for name, collector := range r.collectors {
		result[name] = collector
	}
	return result
}

// CollectFromDevice collects metrics from a device using enabled collectors
// Following cisco_exporter behavior: only include collectors that return valid data
func (r *Registry) CollectFromDevice(
	ctx context.Context,
	client *rpc.Client,
	enabledCollectors DeviceCollectors,
	timestamp time.Time,
) (pmetric.Metrics, error) {
	allMetrics := pmetric.NewMetrics()

	// Collect from each enabled collector
	for name, collector := range r.GetAllCollectors() {
		// Check if collector is enabled
		if !enabledCollectors.IsEnabled(name) {
			continue
		}

		// Check if collector is supported on this device
		if !collector.IsSupported(client) {
			continue
		}

		// Collect metrics from this collector
		metrics, err := collector.Collect(ctx, client, timestamp)
		if err != nil {
			// Log error but continue with other collectors
			// cisco_exporter behavior: skip failed collectors
			continue
		}

		// Check if collector returned any valid metrics
		if metrics.ResourceMetrics().Len() == 0 {
			// cisco_exporter behavior: skip collectors with no data
			continue
		}

		// Merge metrics into the main metrics object
		r.mergeMetrics(allMetrics, metrics)
	}

	return allMetrics, nil
}

// CollectFromDeviceWithTiming collects metrics from a device using enabled collectors and returns timing information
// Following cisco_exporter behavior: only record timing for successful collectors with valid data
// Uses concurrent execution with timeouts to ensure modular independence
func (r *Registry) CollectFromDeviceWithTiming(
	ctx context.Context,
	client *rpc.Client,
	enabledCollectors DeviceCollectors,
	timestamp time.Time,
) (pmetric.Metrics, map[string]time.Duration, error) {
	allMetrics := pmetric.NewMetrics()
	timings := make(map[string]time.Duration)

	// Use concurrent collection with optimized timeouts for better performance
	results := r.collectConcurrentlyWithTimeout(ctx, client, enabledCollectors, timestamp, 10*time.Second)

	// Process results from all collectors
	for _, result := range results {
		if result.Error != nil {
			// Log error but continue with other collectors
			continue
		}

		// Check if collector returned any valid metrics
		if result.Metrics.ResourceMetrics().Len() == 0 {
			// cisco_exporter behavior: don't record timing for collectors with no data
			continue
		}

		// Only record timing for successful collectors with valid data
		timings[result.CollectorName] = result.Duration

		// Merge metrics into the main metrics object
		r.mergeMetrics(allMetrics, result.Metrics)
	}

	return allMetrics, timings, nil
}

// mergeMetrics merges source metrics into destination metrics
func (r *Registry) mergeMetrics(dest, src pmetric.Metrics) {
	srcResourceMetrics := src.ResourceMetrics()

	for i := 0; i < srcResourceMetrics.Len(); i++ {
		srcRM := srcResourceMetrics.At(i)
		destRM := dest.ResourceMetrics().AppendEmpty()
		srcRM.CopyTo(destRM)
	}
}

// CollectorResult represents the result of a collector execution
type CollectorResult struct {
	CollectorName string
	Metrics       pmetric.Metrics
	Error         error
	Duration      time.Duration
}

// collectConcurrentlyWithTimeout collects metrics from multiple collectors concurrently with individual timeouts
// This ensures true modular independence - no collector can block others
func (r *Registry) collectConcurrentlyWithTimeout(
	ctx context.Context,
	client *rpc.Client,
	enabledCollectors DeviceCollectors,
	timestamp time.Time,
	timeout time.Duration,
) []CollectorResult {
	var wg sync.WaitGroup
	results := make([]CollectorResult, 0)
	resultsChan := make(chan CollectorResult, len(r.collectors))

	// Start collection for each enabled collector with individual timeout
	for name, collector := range r.GetAllCollectors() {
		// Check if collector is enabled
		if !enabledCollectors.IsEnabled(name) {
			continue
		}

		// Check if collector is supported on this device
		if !collector.IsSupported(client) {
			continue
		}

		wg.Add(1)
		go func(name string, collector Collector) {
			defer wg.Done()

			// Create timeout context for this collector
			collectorCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			start := time.Now()
			var metrics pmetric.Metrics
			var err error

			// Run collector with timeout protection
			done := make(chan struct{})
			go func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("collector %s panicked: %v", name, r)
					}
					close(done)
				}()
				metrics, err = collector.Collect(collectorCtx, client, timestamp)
			}()

			// Wait for completion or timeout
			select {
			case <-done:
				// Collector completed normally
			case <-collectorCtx.Done():
				// Collector timed out
				err = fmt.Errorf("collector %s timed out after %v", name, timeout)
				metrics = pmetric.NewMetrics() // Return empty metrics on timeout
			}

			duration := time.Since(start)

			resultsChan <- CollectorResult{
				CollectorName: name,
				Metrics:       metrics,
				Error:         err,
				Duration:      duration,
			}
		}(name, collector)
	}

	// Wait for all collectors to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for result := range resultsChan {
		results = append(results, result)
	}

	return results
}

// CollectConcurrently collects metrics from multiple collectors concurrently (legacy method)
func (r *Registry) CollectConcurrently(
	ctx context.Context,
	client *rpc.Client,
	enabledCollectors DeviceCollectors,
	timestamp time.Time,
) ([]CollectorResult, error) {
	results := r.collectConcurrentlyWithTimeout(ctx, client, enabledCollectors, timestamp, 10*time.Second)
	return results, nil
}

// GetEnabledCollectorNames returns the names of enabled collectors
func (r *Registry) GetEnabledCollectorNames(enabledCollectors DeviceCollectors) []string {
	var names []string

	for name := range r.GetAllCollectors() {
		if enabledCollectors.IsEnabled(name) {
			names = append(names, name)
		}
	}

	return names
}
