// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver"

import (
	"fmt"
	"strconv"
	"strings"

	ctypes "github.com/docker/docker/api/types/container"
)

const nanosInASecond = 1e9

func calculateMemoryPercent(limit, usedNoCache uint64) float64 {
	// MemoryStats.Limit will never be 0 unless the container is not running and we haven't
	// got any data from cgroup
	if limit != 0 {
		return float64(usedNoCache) / float64(limit) * 100.0
	}
	return 0.0
}

// calculateCPULimit calculate the number of cpus assigned to a container.
//
// Calculation is based on 3 alternatives by the following order:
// - nanocpus:   if set by i.e docker run -cpus=2
// - cpusetCpus: if set by i.e docker run -docker run -cpuset-cpus="0,2"
// - cpuquota:   if set by i.e docker run -cpu-quota=50000
//
// See https://docs.docker.com/config/containers/resource_constraints/#configure-the-default-cfs-scheduler for background.
func calculateCPULimit(hostConfig *ctypes.HostConfig) (float64, error) {
	var cpuLimit float64
	var err error

	switch {
	case hostConfig.NanoCPUs > 0:
		cpuLimit = float64(hostConfig.NanoCPUs) / nanosInASecond
	case hostConfig.CpusetCpus != "":
		cpuLimit, err = parseCPUSet(hostConfig.CpusetCpus)
		if err != nil {
			return cpuLimit, err
		}
	case hostConfig.CPUQuota > 0:
		period := hostConfig.CPUPeriod
		if period == 0 {
			period = 100000 // Default CFS Period
		}
		cpuLimit = float64(hostConfig.CPUQuota) / float64(period)
	}
	return cpuLimit, nil
}

// parseCPUSet helper function to decompose -cpuset-cpus value into number os cpus.
func parseCPUSet(line string) (float64, error) {
	var numCPUs uint64

	lineSlice := strings.SplitSeq(line, ",")
	for l := range lineSlice {
		lineParts := strings.Split(l, "-")
		if len(lineParts) == 2 {
			p0, err0 := strconv.Atoi(lineParts[0])
			if err0 != nil {
				return 0, fmt.Errorf("invalid -cpuset-cpus value: %w", err0)
			}
			p1, err1 := strconv.Atoi(lineParts[1])
			if err1 != nil {
				return 0, fmt.Errorf("invalid -cpuset-cpus value: %w", err1)
			}
			numCPUs += uint64(p1 - p0 + 1)
		} else if len(lineParts) == 1 {
			numCPUs++
		}
	}
	return float64(numCPUs), nil
}
