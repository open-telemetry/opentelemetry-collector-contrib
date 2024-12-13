// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package extractors

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/testutils"

	"github.com/stretchr/testify/assert"
)

func TestConvertPodToRaw(t *testing.T) {
	result := testutils.LoadKubeletSummary(t, "./testdata/PreSingleKubeletSummary.json")

	podRawMetric := ConvertPodToRaw(result.Pods[0])

	assert.Equal(t, podRawMetric.Id, "01bfbe59-2925-4ad5-a8d3-a1b23e3ddd74")
	assert.Equal(t, podRawMetric.Name, "windows-server-iis-ltsc2019-58d94b5844-6v2pg")
	assert.Equal(t, podRawMetric.Namespace, "amazon-cloudwatch")
	parsedtime, _ := time.Parse(time.RFC3339, "2023-12-21T15:19:59Z")
	assert.Equal(t, podRawMetric.Time, parsedtime.Local())
	assert.Equal(t, podRawMetric.CPUStats.UsageCoreNanoSeconds, uint64(289625000000))
	assert.Equal(t, podRawMetric.CPUStats.UsageNanoCores, uint64(0))

	assert.Equal(t, podRawMetric.MemoryStats.UsageBytes, uint64(0))
	assert.Equal(t, podRawMetric.MemoryStats.AvailableBytes, uint64(0))
	assert.Equal(t, podRawMetric.MemoryStats.WorkingSetBytes, uint64(208949248))
	assert.Equal(t, podRawMetric.MemoryStats.RSSBytes, uint64(0))
	assert.Equal(t, podRawMetric.MemoryStats.PageFaults, uint64(0))
	assert.Equal(t, podRawMetric.MemoryStats.MajorPageFaults, uint64(0))
}

func TestConvertContainerToRaw(t *testing.T) {
	result := testutils.LoadKubeletSummary(t, "./testdata/PreSingleKubeletSummary.json")

	containerRawMetric := ConvertContainerToRaw(result.Pods[0].Containers[0], result.Pods[0])

	assert.Equal(t, containerRawMetric.Id, "01bfbe59-2925-4ad5-a8d3-a1b23e3ddd74-windows-server-iis-ltsc2019")
	assert.Equal(t, containerRawMetric.Name, "windows-server-iis-ltsc2019")
	assert.Equal(t, containerRawMetric.Namespace, "amazon-cloudwatch")
	parsedtime, _ := time.Parse(time.RFC3339, "2023-12-21T15:19:59Z")
	assert.Equal(t, containerRawMetric.Time, parsedtime.Local())
	assert.Equal(t, containerRawMetric.CPUStats.UsageCoreNanoSeconds, uint64(289625000000))
	assert.Equal(t, containerRawMetric.CPUStats.UsageNanoCores, uint64(0))

	assert.Equal(t, containerRawMetric.MemoryStats.UsageBytes, uint64(0))
	assert.Equal(t, containerRawMetric.MemoryStats.AvailableBytes, uint64(0))
	assert.Equal(t, containerRawMetric.MemoryStats.WorkingSetBytes, uint64(208949248))
	assert.Equal(t, containerRawMetric.MemoryStats.RSSBytes, uint64(0))
	assert.Equal(t, containerRawMetric.MemoryStats.PageFaults, uint64(0))
	assert.Equal(t, containerRawMetric.MemoryStats.MajorPageFaults, uint64(0))
}

func TestConvertNodeToRaw(t *testing.T) {
	result := testutils.LoadKubeletSummary(t, "./testdata/PreSingleKubeletSummary.json")

	nodeRawMetric := ConvertNodeToRaw(result.Node)

	assert.Equal(t, nodeRawMetric.Id, "ip-192-168-44-84.us-west-2.compute.internal")
	assert.Equal(t, nodeRawMetric.Name, "ip-192-168-44-84.us-west-2.compute.internal")
	assert.Equal(t, nodeRawMetric.Namespace, "")
	parsedtime, _ := time.Parse(time.RFC3339, "2023-12-21T15:19:58Z")
	assert.Equal(t, nodeRawMetric.Time, parsedtime.Local())
	assert.Equal(t, nodeRawMetric.CPUStats.UsageCoreNanoSeconds, uint64(38907680000000))
	assert.Equal(t, nodeRawMetric.CPUStats.UsageNanoCores, uint64(20000000))

	assert.Equal(t, nodeRawMetric.MemoryStats.UsageBytes, uint64(3583389696))
	assert.Equal(t, nodeRawMetric.MemoryStats.AvailableBytes, uint64(7234662400))
	assert.Equal(t, nodeRawMetric.MemoryStats.WorkingSetBytes, uint64(1040203776))
	assert.Equal(t, nodeRawMetric.MemoryStats.RSSBytes, uint64(0))
	assert.Equal(t, nodeRawMetric.MemoryStats.PageFaults, uint64(0))
	assert.Equal(t, nodeRawMetric.MemoryStats.MajorPageFaults, uint64(0))
}
