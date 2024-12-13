// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package extractors

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	cExtractor "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	cTestUtils "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

	"github.com/stretchr/testify/require"
)

func TestMemStats(t *testing.T) {
	MockCPUMemInfo := cTestUtils.MockCPUMemInfo{}
	result := testutils.LoadKubeletSummary(t, "./testdata/PreSingleKubeletSummary.json")
	result2 := testutils.LoadKubeletSummary(t, "./testdata/CurSingleKubeletSummary.json")

	podRawMetric := ConvertPodToRaw(result.Pods[0])
	podRawMetric2 := ConvertPodToRaw(result2.Pods[0])

	containerType := containerinsight.TypePod
	extractor := NewMemMetricExtractor(nil)

	var cMetrics []*stores.CIMetricImpl
	if extractor.HasValue(podRawMetric) {
		cMetrics = extractor.GetValue(podRawMetric, MockCPUMemInfo, containerType)
	}
	if extractor.HasValue(podRawMetric2) {
		cMetrics = extractor.GetValue(podRawMetric2, MockCPUMemInfo, containerType)
	}

	cExtractor.AssertContainsTaggedUint(t, cMetrics[0], "pod_memory_rss", 0)
	cExtractor.AssertContainsTaggedUint(t, cMetrics[0], "pod_memory_usage", 0)
	cExtractor.AssertContainsTaggedUint(t, cMetrics[0], "pod_memory_working_set", 209088512)

	cExtractor.AssertContainsTaggedFloat(t, cMetrics[0], "pod_memory_pgfault", 0, 0)
	cExtractor.AssertContainsTaggedFloat(t, cMetrics[0], "pod_memory_pgmajfault", 0, 0)
	require.NoError(t, extractor.Shutdown())

	// for node type
	containerType = containerinsight.TypeNode
	extractor = NewMemMetricExtractor(nil)

	nodeRawMetric := ConvertNodeToRaw(result.Node)
	nodeRawMetric2 := ConvertNodeToRaw(result2.Node)

	if extractor.HasValue(nodeRawMetric) {
		cMetrics = extractor.GetValue(nodeRawMetric, MockCPUMemInfo, containerType)
	}

	if extractor.HasValue(nodeRawMetric2) {
		cMetrics = extractor.GetValue(nodeRawMetric2, MockCPUMemInfo, containerType)
	}

	cExtractor.AssertContainsTaggedUint(t, cMetrics[0], "node_memory_rss", 0)
	cExtractor.AssertContainsTaggedUint(t, cMetrics[0], "node_memory_usage", 3572293632)
	cExtractor.AssertContainsTaggedUint(t, cMetrics[0], "node_memory_working_set", 1026678784)
	cExtractor.AssertContainsTaggedInt(t, cMetrics[0], "node_memory_limit", 1073741824)

	cExtractor.AssertContainsTaggedFloat(t, cMetrics[0], "node_memory_pgfault", 0, 0)
	cExtractor.AssertContainsTaggedFloat(t, cMetrics[0], "node_memory_pgmajfault", 0, 0)
	cExtractor.AssertContainsTaggedFloat(t, cMetrics[0], "node_memory_utilization", 95, 0.7)

	require.NoError(t, extractor.Shutdown())
}
