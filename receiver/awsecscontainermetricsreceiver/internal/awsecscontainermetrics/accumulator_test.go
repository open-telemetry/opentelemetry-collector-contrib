// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
	require.NotEmpty(t, acc.mds)
}

func TestGetMetricsDataMissingContainerStats(t *testing.T) {
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001-Missing", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: &f, Memory: &v}},
	}
	acc.getMetricsData(cstats, tm, logger)
	require.NotEmpty(t, acc.mds)
}

func TestGetMetricsDataForStoppedContainer(t *testing.T) {
	cstats = map[string]*ContainerStats{"001": nil}
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", CreatedAt: "2020-07-30T22:12:29.842610987Z", StartedAt: "2020-07-30T22:12:31.842610987Z", FinishedAt: "2020-07-31T22:10:29.842610987Z", KnownStatus: "STOPPED", Limits: ecsutil.Limits{CPU: &f, Memory: &v}},
	}
	acc.getMetricsData(cstats, tm, logger)
	require.NotEmpty(t, acc.mds)
}

func TestWrongFormatTimeDataForStoppedContainer(t *testing.T) {
	cstats = map[string]*ContainerStats{"001": nil}
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", CreatedAt: "2020-07-30T22:12:29.842610987Z", StartedAt: "2020-07-30T22:12:31.842610987Z", FinishedAt: "2020-07-31 22:10:29", KnownStatus: "STOPPED", Limits: ecsutil.Limits{CPU: &f, Memory: &v}},
	}
	acc.getMetricsData(cstats, tm, logger)
	require.NotEmpty(t, acc.mds)
}

func TestGetMetricsDataMissingContainerLimit(t *testing.T) {
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1"},
	}

	acc.getMetricsData(cstats, tm, logger)
	require.NotEmpty(t, acc.mds)
}

func TestGetMetricsDataContainerLimitCpuNil(t *testing.T) {
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: nil, Memory: &v}},
	}

	acc.getMetricsData(cstats, tm, logger)
	require.NotEmpty(t, acc.mds)
}

func TestGetMetricsDataContainerLimitMemoryNil(t *testing.T) {
	tm.Containers = []ecsutil.ContainerMetadata{
		{ContainerName: "container-1", DockerID: "001", DockerName: "docker-container-1", Limits: ecsutil.Limits{CPU: &f, Memory: nil}},
	}

	acc.getMetricsData(cstats, tm, logger)
	require.NotEmpty(t, acc.mds)
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
	require.NotEmpty(t, acc.mds)
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
	require.NotEmpty(t, acc.mds)
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
	require.NotEmpty(t, acc.mds)
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
	require.NotEmpty(t, acc.mds)
}

func TestIsEmptyStats(t *testing.T) {
	require.False(t, isEmptyStats(&containerStats))
	require.True(t, isEmptyStats(cstats["002"]))
	cstats = map[string]*ContainerStats{"001": nil}
	require.True(t, isEmptyStats(cstats["001"]))
	cstats = map[string]*ContainerStats{"001": {}}
	require.True(t, isEmptyStats(cstats["001"]))
}

func TestCalculateDuration(t *testing.T) {
	startTime := "2020-10-02T00:15:07.620912337Z"
	endTime := "2020-10-03T15:14:06.620913372Z"
	result, err := calculateDuration(startTime, endTime)
	require.Equal(t, 140339.000001035, result)
	require.NoError(t, err)

	startTime = "2010-10-02T00:15:07.620912337Z"
	endTime = "2020-10-03T15:14:06.620913372Z"
	result, err = calculateDuration(startTime, endTime)
	require.Equal(t, 3.15759539000001e+08, result)
	require.NoError(t, err)

	startTime = "2010-10-02 00:15:07"
	endTime = "2020-10-03T15:14:06.620913372Z"
	result, err = calculateDuration(startTime, endTime)
	require.Error(t, err)
	require.EqualValues(t, 0, result)

	startTime = "2010-10-02T00:15:07.620912337Z"
	endTime = "2020-10-03 15:14:06 +800"
	result, err = calculateDuration(startTime, endTime)
	require.Error(t, err)
	require.EqualValues(t, 0, result)
}

func TestGetContainerStartTime(t *testing.T) {
	// StartedAt present and valid
	cm := &ecsutil.ContainerMetadata{
		StartedAt: "2020-07-30T22:12:31.138830877Z",
		CreatedAt: "2020-07-30T22:12:29.837074927Z",
	}
	expectedTime, err := time.Parse(time.RFC3339Nano, cm.StartedAt)
	require.NoError(t, err)
	assert.Equal(t, pcommon.NewTimestampFromTime(expectedTime), getContainerStartTime(cm))

	// StartedAt empty, fall back to CreatedAt
	cm = &ecsutil.ContainerMetadata{
		CreatedAt: "2020-07-30T22:12:29.837074927Z",
	}
	expectedCreated, err := time.Parse(time.RFC3339Nano, cm.CreatedAt)
	require.NoError(t, err)
	assert.Equal(t, pcommon.NewTimestampFromTime(expectedCreated), getContainerStartTime(cm))

	// StartedAt invalid, fall back to CreatedAt
	cm = &ecsutil.ContainerMetadata{
		StartedAt: "invalid-time",
		CreatedAt: "2020-07-30T22:12:29.837074927Z",
	}
	assert.Equal(t, pcommon.NewTimestampFromTime(expectedCreated), getContainerStartTime(cm))

	// Neither present
	cm = &ecsutil.ContainerMetadata{}
	assert.Equal(t, pcommon.Timestamp(0), getContainerStartTime(cm))
}

func TestGetTaskStartTime(t *testing.T) {
	// Multiple containers: earliest StartedAt selected
	metadata := ecsutil.TaskMetadata{
		PullStartedAt: "2020-07-30T22:12:25.705983342Z",
		Containers: []ecsutil.ContainerMetadata{
			{StartedAt: "2020-07-30T22:12:31.138830877Z"},
			{StartedAt: "2020-07-30T22:12:30.956687010Z"},
			{StartedAt: "2020-07-30T22:12:31.153459485Z"},
		},
	}
	expectedEarliest, err := time.Parse(time.RFC3339Nano, "2020-07-30T22:12:30.956687010Z")
	require.NoError(t, err)
	assert.Equal(t, pcommon.NewTimestampFromTime(expectedEarliest), getTaskStartTime(metadata))

	// No containers have StartedAt, fall back to PullStartedAt
	metadata = ecsutil.TaskMetadata{
		PullStartedAt: "2020-07-30T22:12:25.705983342Z",
		Containers: []ecsutil.ContainerMetadata{
			{ContainerName: "c1"},
			{ContainerName: "c2"},
		},
	}
	expectedPull, err := time.Parse(time.RFC3339Nano, "2020-07-30T22:12:25.705983342Z")
	require.NoError(t, err)
	assert.Equal(t, pcommon.NewTimestampFromTime(expectedPull), getTaskStartTime(metadata))

	// No StartedAt and no PullStartedAt, fall back to earliest CreatedAt
	metadata = ecsutil.TaskMetadata{
		Containers: []ecsutil.ContainerMetadata{
			{CreatedAt: "2020-07-30T22:12:29.837074927Z"},
			{CreatedAt: "2020-07-30T22:12:28.123456789Z"},
		},
	}
	expectedCreated, err := time.Parse(time.RFC3339Nano, "2020-07-30T22:12:28.123456789Z")
	require.NoError(t, err)
	assert.Equal(t, pcommon.NewTimestampFromTime(expectedCreated), getTaskStartTime(metadata))

	// Empty metadata
	assert.Equal(t, pcommon.Timestamp(0), getTaskStartTime(ecsutil.TaskMetadata{}))
}

func TestGetMetricsDataCumulativeSumsWithStartTimestampAndMonotonic(t *testing.T) {
	taskMeta := ecsutil.TaskMetadata{
		Cluster:  "cluster-1",
		TaskARN:  "arn:aws:some-value/001",
		Family:   "task-def-family-1",
		Revision: "task-def-version",
		Containers: []ecsutil.ContainerMetadata{
			{
				ContainerName: "container-1",
				DockerID:      "001",
				DockerName:    "docker-container-1",
				StartedAt:     "2020-07-30T22:12:31.138830877Z",
				Limits:        ecsutil.Limits{CPU: &f, Memory: &v},
			},
		},
		Limits: ecsutil.Limits{CPU: &f, Memory: &v},
	}

	testAcc := &metricDataAccumulator{}
	testAcc.getMetricsData(cstats, taskMeta, logger)
	require.NotEmpty(t, testAcc.mds)

	expectedStartTime, err := time.Parse(time.RFC3339Nano, "2020-07-30T22:12:31.138830877Z")
	require.NoError(t, err)
	expectedTs := pcommon.NewTimestampFromTime(expectedStartTime)

	sumCount := 0
	for _, md := range testAcc.mds {
		rms := md.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			sms := rms.At(i).ScopeMetrics()
			for j := 0; j < sms.Len(); j++ {
				metrics := sms.At(j).Metrics()
				for k := 0; k < metrics.Len(); k++ {
					m := metrics.At(k)
					if m.Type() == pmetric.MetricTypeSum {
						sumCount++
						assert.True(t, m.Sum().IsMonotonic(), "metric %s should be monotonic", m.Name())
						assert.Equal(t, pmetric.AggregationTemporalityCumulative, m.Sum().AggregationTemporality())
						require.Equal(t, 1, m.Sum().DataPoints().Len())
						assert.Equal(t, expectedTs, m.Sum().DataPoints().At(0).StartTimestamp())
					}
				}
			}
		}
	}
	assert.Greater(t, sumCount, 0)
}
