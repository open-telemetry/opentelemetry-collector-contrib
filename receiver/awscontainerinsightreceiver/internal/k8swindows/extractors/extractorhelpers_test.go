// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package extractors

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/testutils"
)

func TestConvertPodToRaw(t *testing.T) {
	result := testutils.LoadKubeletSummary(t, "./testdata/PreSingleKubeletSummary.json")

	podRawMetric := ConvertPodToRaw(result.Pods[0])

	assert.Equal(t, "01bfbe59-2925-4ad5-a8d3-a1b23e3ddd74", podRawMetric.Id)
	assert.Equal(t, "windows-server-iis-ltsc2019-58d94b5844-6v2pg", podRawMetric.Name)
	assert.Equal(t, "amazon-cloudwatch", podRawMetric.Namespace)
	parsedtime, _ := time.Parse(time.RFC3339, "2023-12-21T15:19:59Z")
	assert.Equal(t, parsedtime.Local(), podRawMetric.Time)
	assert.Equal(t, uint64(289625000000), podRawMetric.CPUStats.UsageCoreNanoSeconds)
	assert.Equal(t, uint64(0), podRawMetric.CPUStats.UsageNanoCores)

	assert.Equal(t, uint64(0), podRawMetric.MemoryStats.UsageBytes)
	assert.Equal(t, uint64(0), podRawMetric.MemoryStats.AvailableBytes)
	assert.Equal(t, uint64(208949248), podRawMetric.MemoryStats.WorkingSetBytes)
	assert.Equal(t, uint64(0), podRawMetric.MemoryStats.RSSBytes)
	assert.Equal(t, uint64(0), podRawMetric.MemoryStats.PageFaults)
	assert.Equal(t, uint64(0), podRawMetric.MemoryStats.MajorPageFaults)
}

func TestConvertContainerToRaw(t *testing.T) {
	result := testutils.LoadKubeletSummary(t, "./testdata/PreSingleKubeletSummary.json")

	containerRawMetric := ConvertContainerToRaw(result.Pods[0].Containers[0], result.Pods[0])

	assert.Equal(t, "01bfbe59-2925-4ad5-a8d3-a1b23e3ddd74-windows-server-iis-ltsc2019", containerRawMetric.Id)
	assert.Equal(t, "windows-server-iis-ltsc2019", containerRawMetric.Name)
	assert.Equal(t, "amazon-cloudwatch", containerRawMetric.Namespace)
	parsedtime, _ := time.Parse(time.RFC3339, "2023-12-21T15:19:59Z")
	assert.Equal(t, parsedtime.Local(), containerRawMetric.Time)
	assert.Equal(t, uint64(289625000000), containerRawMetric.CPUStats.UsageCoreNanoSeconds)
	assert.Equal(t, uint64(0), containerRawMetric.CPUStats.UsageNanoCores)

	assert.Equal(t, uint64(0), containerRawMetric.MemoryStats.UsageBytes)
	assert.Equal(t, uint64(0), containerRawMetric.MemoryStats.AvailableBytes)
	assert.Equal(t, uint64(208949248), containerRawMetric.MemoryStats.WorkingSetBytes)
	assert.Equal(t, uint64(0), containerRawMetric.MemoryStats.RSSBytes)
	assert.Equal(t, uint64(0), containerRawMetric.MemoryStats.PageFaults)
	assert.Equal(t, uint64(0), containerRawMetric.MemoryStats.MajorPageFaults)
}

func TestConvertNodeToRaw(t *testing.T) {
	result := testutils.LoadKubeletSummary(t, "./testdata/PreSingleKubeletSummary.json")

	nodeRawMetric := ConvertNodeToRaw(result.Node)

	assert.Equal(t, "ip-192-168-44-84.us-west-2.compute.internal", nodeRawMetric.Id)
	assert.Equal(t, "ip-192-168-44-84.us-west-2.compute.internal", nodeRawMetric.Name)
	assert.Equal(t, "", nodeRawMetric.Namespace)
	parsedtime, _ := time.Parse(time.RFC3339, "2023-12-21T15:19:58Z")
	assert.Equal(t, parsedtime.Local(), nodeRawMetric.Time)
	assert.Equal(t, uint64(38907680000000), nodeRawMetric.CPUStats.UsageCoreNanoSeconds)
	assert.Equal(t, uint64(20000000), nodeRawMetric.CPUStats.UsageNanoCores)

	assert.Equal(t, uint64(3583389696), nodeRawMetric.MemoryStats.UsageBytes)
	assert.Equal(t, uint64(7234662400), nodeRawMetric.MemoryStats.AvailableBytes)
	assert.Equal(t, uint64(1040203776), nodeRawMetric.MemoryStats.WorkingSetBytes)
	assert.Equal(t, uint64(0), nodeRawMetric.MemoryStats.RSSBytes)
	assert.Equal(t, uint64(0), nodeRawMetric.MemoryStats.PageFaults)
	assert.Equal(t, uint64(0), nodeRawMetric.MemoryStats.MajorPageFaults)
}
