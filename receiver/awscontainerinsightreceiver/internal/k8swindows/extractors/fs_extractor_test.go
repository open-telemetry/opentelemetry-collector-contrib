// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package extractors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	cExtractor "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

func TestFSStats(t *testing.T) {
	result := testutils.LoadKubeletSummary(t, "./testdata/PreSingleKubeletSummary.json")

	nodeRawMetric := ConvertNodeToRaw(result.Node)

	// node type
	containerType := containerinsight.TypeNode
	extractor := NewFileSystemMetricExtractor(nil)

	var cMetrics []*stores.CIMetricImpl
	if extractor.HasValue(nodeRawMetric) {
		cMetrics = extractor.GetValue(nodeRawMetric, nil, containerType)
	}
	fmt.Println(len(cMetrics))
	expectedFields := map[string]any{
		"node_filesystem_usage":       uint64(34667089920),
		"node_filesystem_capacity":    uint64(85897244672),
		"node_filesystem_available":   uint64(51230154752),
		"node_filesystem_utilization": float64(40.358791544917224),
	}
	expectedTags := map[string]string{
		"fstype": "",
		"device": "",
		"Type":   "NodeFS",
	}
	cExtractor.AssertContainsTaggedField(t, cMetrics[0], expectedFields, expectedTags)

	// pod type
	containerType = containerinsight.TypePod
	extractor = NewFileSystemMetricExtractor(nil)
	podRawMetric := ConvertPodToRaw(result.Pods[0])

	if extractor.HasValue(podRawMetric) {
		cMetrics = extractor.GetValue(podRawMetric, nil, containerType)
	}

	assert.Empty(t, cMetrics)

	// container type for eks
	containerType = containerinsight.TypeContainer
	extractor = NewFileSystemMetricExtractor(nil)
	containerRawMetric := ConvertContainerToRaw(result.Pods[0].Containers[0], result.Pods[0])

	if extractor.HasValue(containerRawMetric) {
		cMetrics = extractor.GetValue(containerRawMetric, nil, containerType)
	}

	expectedFields = map[string]any{
		"container_filesystem_available":   uint64(51230154752),
		"container_filesystem_capacity":    uint64(85897244672),
		"container_filesystem_usage":       uint64(339738624),
		"container_filesystem_utilization": float64(0.3955174875484043),
	}
	expectedTags = map[string]string{
		"fstype": "",
		"device": "",
		"Type":   "ContainerFS",
	}
	cExtractor.AssertContainsTaggedField(t, cMetrics[0], expectedFields, expectedTags)

	expectedFields = map[string]any{
		"container_filesystem_available":   uint64(51230154752),
		"container_filesystem_capacity":    uint64(85897244672),
		"container_filesystem_usage":       uint64(919463),
		"container_filesystem_utilization": float64(0.0010704219949207732),
	}
	expectedTags = map[string]string{
		"fstype": "",
		"device": "",
		"Type":   "ContainerFS",
	}
	cExtractor.AssertContainsTaggedField(t, cMetrics[1], expectedFields, expectedTags)
}
