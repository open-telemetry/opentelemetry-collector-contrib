// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package awsecscontainermetrics

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
)

func TestMetricData(t *testing.T) {
	v := uint64(1)
	f := 1.0

	memStats := make(map[string]uint64)
	memStats["cache"] = v

	mem := MemoryStats{
		Usage:          &v,
		MaxUsage:       &v,
		Limit:          &v,
		MemoryReserved: &v,
		MemoryUtilized: &v,
		Stats:          memStats,
	}

	disk := DiskStats{
		IoServiceBytesRecursives: []IoServiceBytesRecursive{
			{Op: "Read", Value: &v},
			{Op: "Write", Value: &v},
			{Op: "Total", Value: &v},
		},
	}

	net := make(map[string]NetworkStats)
	net["eth0"] = NetworkStats{
		RxBytes:   &v,
		RxPackets: &v,
		RxErrors:  &v,
		RxDropped: &v,
		TxBytes:   &v,
		TxPackets: &v,
		TxErrors:  &v,
		TxDropped: &v,
	}

	netRate := NetworkRateStats{
		RxBytesPerSecond: &f,
		TxBytesPerSecond: &f,
	}

	percpu := []*uint64{&v, &v}
	cpuUsage := CPUUsage{
		TotalUsage:        &v,
		UsageInKernelmode: &v,
		UsageInUserMode:   &v,
		PerCPUUsage:       percpu,
	}

	cpuStats := CPUStats{
		CPUUsage:       &cpuUsage,
		OnlineCpus:     &v,
		SystemCPUUsage: &v,
		CPUUtilized:    &v,
		CPUReserved:    &v,
	}
	containerStats := ContainerStats{
		Name:        "test",
		ID:          "001",
		Memory:      &mem,
		Disk:        &disk,
		Network:     net,
		NetworkRate: &netRate,
		CPU:         &cpuStats,
	}

	tm := ecsutil.TaskMetadata{
		Cluster:  "cluster-1",
		TaskARN:  "arn:aws:some-value/001",
		Family:   "task-def-family-1",
		Revision: "task-def-version",
		Containers: []ecsutil.ContainerMetadata{
			{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: &f, Memory: &v}},
		},
		Limits: ecsutil.Limits{CPU: &f, Memory: &v},
	}

	cstats := make(map[string]*ContainerStats)
	cstats["001"] = &containerStats

	logger := zap.NewNop()
	md := MetricsData(cstats, tm, logger)
	require.NotEmpty(t, md)
}
