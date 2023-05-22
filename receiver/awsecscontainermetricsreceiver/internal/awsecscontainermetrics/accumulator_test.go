// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
)

var (
	v         = uint64(1)
	f         = 1.0
	floatZero = float64(0)
	logger    = zap.NewNop()

	memStats = map[string]uint64{"cache": v}

	mem = MemoryStats{
		Usage:          &v,
		MaxUsage:       &v,
		Limit:          &v,
		MemoryReserved: &v,
		MemoryUtilized: &v,
		Stats:          memStats,
	}

	disk = DiskStats{
		IoServiceBytesRecursives: []IoServiceBytesRecursive{
			{Op: "Read", Value: &v},
			{Op: "Write", Value: &v},
			{Op: "Total", Value: &v},
		},
	}

	networkStat = NetworkStats{
		RxBytes:   &v,
		RxPackets: &v,
		RxErrors:  &v,
		RxDropped: &v,
		TxBytes:   &v,
		TxPackets: &v,
		TxErrors:  &v,
		TxDropped: &v,
	}
	net = map[string]NetworkStats{"eth0": networkStat}

	netRate = NetworkRateStats{
		RxBytesPerSecond: &f,
		TxBytesPerSecond: &f,
	}

	percpu   = []*uint64{&v, &v}
	cpuUsage = CPUUsage{
		TotalUsage:        &v,
		UsageInKernelmode: &v,
		UsageInUserMode:   &v,
		PerCPUUsage:       percpu,
	}

	cpuStats = CPUStats{
		CPUUsage:       &cpuUsage,
		OnlineCpus:     &v,
		SystemCPUUsage: &v,
		CPUUtilized:    &v,
		CPUReserved:    &v,
	}
	containerStats = ContainerStats{
		Name:        "test",
		ID:          "001",
		Memory:      &mem,
		Disk:        &disk,
		Network:     net,
		NetworkRate: &netRate,
		CPU:         &cpuStats,
	}

	tm = ecsutil.TaskMetadata{
		Cluster:  "cluster-1",
		TaskARN:  "arn:aws:some-value/001",
		Family:   "task-def-family-1",
		Revision: "task-def-version",
		Containers: []ecsutil.ContainerMetadata{
			{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: &f, Memory: &v}},
		},
		Limits: ecsutil.Limits{CPU: &f, Memory: &v},
	}

	cstats = map[string]*ContainerStats{"001": &containerStats}
	acc    = metricDataAccumulator{
		mds: nil,
	}
)

func TestGetMetricsDataAllValid(t *testing.T) {
	acc.getMetricsData(cstats, tm, logger)
	require.Less(t, 0, len(acc.mds))
}

func TestGetMetricsDataMissingContainerStats(t *testing.T) {
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001-Missing", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: &f, Memory: &v}},
	}
	acc.getMetricsData(cstats, tm, logger)
	require.Less(t, 0, len(acc.mds))
}

func TestGetMetricsDataForStoppedContainer(t *testing.T) {
	cstats = map[string]*ContainerStats{"001": nil}
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", CreatedAt: "2020-07-30T22:12:29.842610987Z", StartedAt: "2020-07-30T22:12:31.842610987Z", FinishedAt: "2020-07-31T22:10:29.842610987Z", KnownStatus: "STOPPED", Limits: ecsutil.Limits{CPU: &f, Memory: &v}},
	}
	acc.getMetricsData(cstats, tm, logger)
	require.Less(t, 0, len(acc.mds))
}

func TestWrongFormatTimeDataForStoppedContainer(t *testing.T) {
	cstats = map[string]*ContainerStats{"001": nil}
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", CreatedAt: "2020-07-30T22:12:29.842610987Z", StartedAt: "2020-07-30T22:12:31.842610987Z", FinishedAt: "2020-07-31 22:10:29", KnownStatus: "STOPPED", Limits: ecsutil.Limits{CPU: &f, Memory: &v}},
	}
	acc.getMetricsData(cstats, tm, logger)
	require.Less(t, 0, len(acc.mds))
}

func TestGetMetricsDataMissingContainerLimit(t *testing.T) {
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1"},
	}

	acc.getMetricsData(cstats, tm, logger)
	require.Less(t, 0, len(acc.mds))
}

func TestGetMetricsDataContainerLimitCpuNil(t *testing.T) {
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: nil, Memory: &v}},
	}

	acc.getMetricsData(cstats, tm, logger)
	require.Less(t, 0, len(acc.mds))
}

func TestGetMetricsDataContainerLimitMemoryNil(t *testing.T) {
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: &f, Memory: nil}},
	}

	acc.getMetricsData(cstats, tm, logger)
	require.Less(t, 0, len(acc.mds))
}

func TestGetMetricsDataMissingTaskLimit(t *testing.T) {
	tm = ecsutil.TaskMetadata{
		Cluster:  "cluster-1",
		TaskARN:  "arn:aws:some-value/001",
		Family:   "task-def-family-1",
		Revision: "task-def-version",
		Containers: []ecsutil.ContainerMetadata{
			{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: &f, Memory: &v}},
		},
	}

	acc.getMetricsData(cstats, tm, logger)
	require.Less(t, 0, len(acc.mds))
}

func TestGetMetricsDataTaskLimitCpuNil(t *testing.T) {
	tm = ecsutil.TaskMetadata{
		Cluster:  "cluster-1",
		TaskARN:  "arn:aws:some-value/001",
		Family:   "task-def-family-1",
		Revision: "task-def-version",
		Containers: []ecsutil.ContainerMetadata{
			{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: &f, Memory: &v}},
		},
		Limits: ecsutil.Limits{CPU: nil, Memory: &v},
	}

	acc.getMetricsData(cstats, tm, logger)
	require.Less(t, 0, len(acc.mds))
}

func TestGetMetricsDataTaskLimitMemoryNil(t *testing.T) {
	tm = ecsutil.TaskMetadata{
		Cluster:  "cluster-1",
		TaskARN:  "arn:aws:some-value/001",
		Family:   "task-def-family-1",
		Revision: "task-def-version",
		Containers: []ecsutil.ContainerMetadata{
			{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: &f, Memory: &v}},
		},
		Limits: ecsutil.Limits{CPU: &f, Memory: nil},
	}

	acc.getMetricsData(cstats, tm, logger)
	require.Less(t, 0, len(acc.mds))
}

func TestGetMetricsDataCpuReservedZero(t *testing.T) {
	tm = ecsutil.TaskMetadata{
		Cluster:  "cluster-1",
		TaskARN:  "arn:aws:some-value/001",
		Family:   "task-def-family-1",
		Revision: "task-def-version",
		Containers: []ecsutil.ContainerMetadata{
			{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: &floatZero, Memory: nil}},
		},
		Limits: ecsutil.Limits{CPU: &floatZero, Memory: &v},
	}

	acc.getMetricsData(cstats, tm, logger)
	require.Less(t, 0, len(acc.mds))
}
func TestIsEmptyStats(t *testing.T) {
	require.EqualValues(t, false, isEmptyStats(&containerStats))
	require.EqualValues(t, true, isEmptyStats(cstats["002"]))
	cstats = map[string]*ContainerStats{"001": nil}
	require.EqualValues(t, true, isEmptyStats(cstats["001"]))
	cstats = map[string]*ContainerStats{"001": {}}
	require.EqualValues(t, true, isEmptyStats(cstats["001"]))
}

func TestCalculateDuration(t *testing.T) {

	startTime := "2020-10-02T00:15:07.620912337Z"
	endTime := "2020-10-03T15:14:06.620913372Z"
	result, err := calculateDuration(startTime, endTime)
	require.EqualValues(t, 140339.000001035, result)
	require.NoError(t, err)

	startTime = "2010-10-02T00:15:07.620912337Z"
	endTime = "2020-10-03T15:14:06.620913372Z"
	result, err = calculateDuration(startTime, endTime)
	require.EqualValues(t, 3.15759539000001e+08, result)
	require.NoError(t, err)

	startTime = "2010-10-02 00:15:07"
	endTime = "2020-10-03T15:14:06.620913372Z"
	result, err = calculateDuration(startTime, endTime)
	require.NotNil(t, err)
	require.EqualValues(t, 0, result)

	startTime = "2010-10-02T00:15:07.620912337Z"
	endTime = "2020-10-03 15:14:06 +800"
	result, err = calculateDuration(startTime, endTime)
	require.NotNil(t, err)
	require.EqualValues(t, 0, result)

}
