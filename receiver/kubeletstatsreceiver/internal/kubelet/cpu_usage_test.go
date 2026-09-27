// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestCPUUsageCalculator_FirstSampleHasNoUsage(t *testing.T) {
	c := NewCPUUsageCalculator()

	c.startScrape()
	_, ok := c.calculateUsage("key", 10, pcommon.Timestamp(1_000_000_000))
	c.endScrape()

	require.False(t, ok)
}

func TestCPUUsageCalculator_CalculatesRate(t *testing.T) {
	c := NewCPUUsageCalculator()

	c.startScrape()
	_, ok := c.calculateUsage("key", 10, pcommon.Timestamp(0))
	c.endScrape()
	require.False(t, ok)

	// 4 CPU-seconds elapsed over 2 wall-clock seconds -> 2 cores.
	c.startScrape()
	usage, ok := c.calculateUsage("key", 14, pcommon.Timestamp(2_000_000_000))
	c.endScrape()

	require.True(t, ok)
	require.InDelta(t, 2.0, usage, 1e-9)
}

func TestCPUUsageCalculator_NonPositiveElapsedTimeIsSkipped(t *testing.T) {
	c := NewCPUUsageCalculator()

	c.startScrape()
	c.calculateUsage("key", 10, pcommon.Timestamp(1_000_000_000))
	c.endScrape()

	// Same sample time as before: elapsed == 0.
	c.startScrape()
	_, ok := c.calculateUsage("key", 12, pcommon.Timestamp(1_000_000_000))
	c.endScrape()
	require.False(t, ok)

	// Sample time went backwards: elapsed < 0.
	c.startScrape()
	_, ok = c.calculateUsage("key", 14, pcommon.Timestamp(500_000_000))
	c.endScrape()
	require.False(t, ok)
}

func TestCPUUsageCalculator_NegativeDeltaIsSkipped(t *testing.T) {
	c := NewCPUUsageCalculator()

	c.startScrape()
	c.calculateUsage("key", 10, pcommon.Timestamp(1_000_000_000))
	c.endScrape()

	// The cumulative counter went backwards, e.g. because the container restarted.
	c.startScrape()
	_, ok := c.calculateUsage("key", 5, pcommon.Timestamp(2_000_000_000))
	c.endScrape()
	require.False(t, ok)
}

func TestCPUUsageCalculator_StaleKeysArePruned(t *testing.T) {
	c := NewCPUUsageCalculator()

	c.startScrape()
	c.calculateUsage("key", 10, pcommon.Timestamp(1_000_000_000))
	c.endScrape()
	require.Len(t, c.previous, 1)

	// The resource is absent from this scrape: nothing touches "key".
	c.startScrape()
	c.endScrape()
	require.Empty(t, c.previous)

	// If the resource reappears later, it is treated as a first sample again.
	c.startScrape()
	_, ok := c.calculateUsage("key", 20, pcommon.Timestamp(3_000_000_000))
	c.endScrape()
	require.False(t, ok)
}

func TestCPUUsageCalculator_MultipleKeysAreIndependent(t *testing.T) {
	c := NewCPUUsageCalculator()

	c.startScrape()
	c.calculateUsage("a", 10, pcommon.Timestamp(0))
	c.calculateUsage("b", 100, pcommon.Timestamp(0))
	c.endScrape()

	c.startScrape()
	usageA, okA := c.calculateUsage("a", 12, pcommon.Timestamp(1_000_000_000))
	usageB, okB := c.calculateUsage("b", 108, pcommon.Timestamp(1_000_000_000))
	c.endScrape()

	require.True(t, okA)
	require.InDelta(t, 2.0, usageA, 1e-9)
	require.True(t, okB)
	require.InDelta(t, 8.0, usageB, 1e-9)
}
