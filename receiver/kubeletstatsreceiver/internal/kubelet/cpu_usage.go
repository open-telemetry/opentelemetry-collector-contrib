// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// cpuUsageSample stores the cumulative CPU time (in seconds) reported for a resource
// at a given point in time, as observed on a previous scrape.
type cpuUsageSample struct {
	cpuTimeSeconds float64
	sampleTime     pcommon.Timestamp
}

// CPUUsageCalculator calculates CPU usage (in cores) as the rate of change of a
// cumulative CPU time counter between two consecutive scrapes:
//
//	usageCores = (cpuTimeEnd - cpuTimeStart) / elapsedSeconds
//
// This mirrors the approach used by the hostmetrics receiver to calculate CPU
// utilization from cumulative CPU time. Samples are keyed by an arbitrary
// per-resource key (e.g. node name, pod UID, or container UID+name) so that a single
// calculator instance can be shared across all node/pod/container/system-container
// resources scraped by the receiver.
type CPUUsageCalculator struct {
	previous map[string]cpuUsageSample
	touched  map[string]bool
}

// NewCPUUsageCalculator creates a new, empty CPUUsageCalculator.
func NewCPUUsageCalculator() *CPUUsageCalculator {
	return &CPUUsageCalculator{
		previous: make(map[string]cpuUsageSample),
	}
}

// startScrape prepares the calculator for a new scrape. It must be called once before
// any calls to calculateUsage for a given scrape, and paired with a following call to
// endScrape once the scrape completes.
func (c *CPUUsageCalculator) startScrape() {
	c.touched = make(map[string]bool)
}

// endScrape prunes stored samples for any resource key that was not observed during
// the scrape (e.g. because the pod/container no longer exists), to avoid unbounded
// growth of the underlying map as resources come and go over time.
func (c *CPUUsageCalculator) endScrape() {
	for key := range c.previous {
		if !c.touched[key] {
			delete(c.previous, key)
		}
	}
	c.touched = nil
}

// calculateUsage returns the CPU usage in cores for the resource identified by key,
// computed as the rate of change of cpuTimeSeconds (a cumulative CPU-seconds counter)
// between sampleTime and the previous sample's time for the same key.
//
// It returns ok=false when there is no previous sample for key (e.g. on the first
// scrape after startup, or after the resource disappeared and reappeared), when the
// elapsed time between samples is not strictly positive (e.g. the kubelet reported a
// stale or duplicate sample time), or when the counter decreased (e.g. a container
// restart reset it). In all of these cases no usage value should be recorded for this
// scrape.
func (c *CPUUsageCalculator) calculateUsage(key string, cpuTimeSeconds float64, sampleTime pcommon.Timestamp) (usageCores float64, ok bool) {
	c.touched[key] = true

	prev, hasPrev := c.previous[key]
	c.previous[key] = cpuUsageSample{cpuTimeSeconds: cpuTimeSeconds, sampleTime: sampleTime}
	if !hasPrev {
		return 0, false
	}

	elapsedSeconds := time.Duration(sampleTime - prev.sampleTime).Seconds()
	if elapsedSeconds <= 0 {
		return 0, false
	}

	delta := cpuTimeSeconds - prev.cpuTimeSeconds
	if delta < 0 {
		return 0, false
	}

	return delta / elapsedSeconds, true
}
