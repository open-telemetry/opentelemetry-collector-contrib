// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package extractors

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	cExtractor "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	cTestUtils "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

func TestCPUStats(t *testing.T) {
	MockCPUMemInfo := cTestUtils.MockCPUMemInfo{}

	result := testutils.LoadKubeletSummary(t, "./testdata/PreSingleKubeletSummary.json")
	result2 := testutils.LoadKubeletSummary(t, "./testdata/CurSingleKubeletSummary.json")

	podRawMetric := ConvertPodToRaw(result.Pods[0])
	podRawMetric2 := ConvertPodToRaw(result2.Pods[0])

	// test container type
	containerType := containerinsight.TypePod
	extractor := NewCPUMetricExtractor(&zap.Logger{})

	var cMetrics []*stores.CIMetricImpl
	if extractor.HasValue(podRawMetric) {
		cMetrics = extractor.GetValue(podRawMetric, MockCPUMemInfo, containerType)
	}
	if extractor.HasValue(podRawMetric2) {
		cMetrics = extractor.GetValue(podRawMetric2, MockCPUMemInfo, containerType)
	}

	cExtractor.AssertContainsTaggedFloat(t, cMetrics[0], "pod_cpu_usage_total", 3.125000, 0)
	cExtractor.AssertContainsTaggedFloat(t, cMetrics[0], "pod_cpu_utilization", 0.156250, 0)
	require.NoError(t, extractor.Shutdown())

	// test node type
	containerType = containerinsight.TypeNode
	extractor = NewCPUMetricExtractor(nil)

	nodeRawMetric := ConvertNodeToRaw(result.Node)
	nodeRawMetric2 := ConvertNodeToRaw(result2.Node)
	if extractor.HasValue(nodeRawMetric) {
		cMetrics = extractor.GetValue(nodeRawMetric, MockCPUMemInfo, containerType)
	}
	if extractor.HasValue(nodeRawMetric2) {
		cMetrics = extractor.GetValue(nodeRawMetric2, MockCPUMemInfo, containerType)
	}

	cExtractor.AssertContainsTaggedFloat(t, cMetrics[0], "node_cpu_usage_total", 51.5, 0.5)
	cExtractor.AssertContainsTaggedFloat(t, cMetrics[0], "node_cpu_utilization", 2.5, 0.5)
	cExtractor.AssertContainsTaggedInt(t, cMetrics[0], "node_cpu_limit", 2000)

	require.NoError(t, extractor.Shutdown())
}
