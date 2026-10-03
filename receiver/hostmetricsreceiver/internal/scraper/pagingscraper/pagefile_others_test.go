// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows && !linux

package pagingscraper

import (
	"testing"

	"github.com/shirou/gopsutil/v4/mem"
	"github.com/stretchr/testify/require"
)

// TestGetPageFileStatsSetsTotalBytes guards against pageFileStats.totalBytes being left
// at its zero value, which caused system.paging.utilization to divide by zero (producing
// +Inf/NaN data points) in paging_scraper_others.go. totalBytes must mirror the same
// vmem.SwapTotal value that usedBytes and freeBytes are already derived from.
func TestGetPageFileStatsSetsTotalBytes(t *testing.T) {
	vmem, err := mem.VirtualMemory()
	require.NoError(t, err)

	stats, err := getPageFileStats()
	require.NoError(t, err)
	require.Len(t, stats, 1)

	require.Equal(t, vmem.SwapTotal, stats[0].totalBytes)
}
