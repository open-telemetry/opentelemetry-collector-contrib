// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	ctypes "github.com/docker/docker/api/types/container"
)

// Following functions has been copied from: calculateCPUPercentWindows()
// https://github.com/docker/cli/blob/f40caed86c0f90d4295702e248c4824cd75839f1/cli/command/container/stats_helpers.go

// Copyright 2012-2017 Docker, Inc.
// This product includes software developed at Docker, Inc. (https://www.docker.com).
// The following is courtesy of our legal counsel:
// Use and transfer of Docker may be subject to certain restrictions by the
// United States and other governments.
// It is your responsibility to ensure that your use and/or transfer does not
// violate applicable laws.
// For more information, please see https://www.bis.doc.gov
// See also https://www.apache.org/dev/crypto.html and/or seek legal counsel.

func calculateCPUPercent(v *ctypes.StatsResponse) float64 {
	// Max number of 100ns intervals between the previous time read and now
	possIntervals := uint64(v.Read.Sub(v.PreRead).Nanoseconds()) // Start with number of ns intervals
	possIntervals /= 100                                         // Convert to number of 100ns intervals
	possIntervals *= uint64(v.NumProcs)                          // Multiple by the number of processors

	// Intervals used
	intervalsUsed := v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage

	// Percentage avoiding divide-by-zero
	if possIntervals > 0 {
		return float64(intervalsUsed) / float64(possIntervals) * 100.0
	}
	return 0.00
}

// calculateMemUsageNoCache calculate memory usage of the container.
func calculateMemUsageNoCache(memoryStats *ctypes.MemoryStats) uint64 {
	return memoryStats.PrivateWorkingSet
}
