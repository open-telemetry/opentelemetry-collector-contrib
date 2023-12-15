// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"
)

func TestCPUStats(t *testing.T) {
	MockCPUMemInfo := testutils.MockCPUMemInfo{}

	result := testutils.LoadContainerInfo(t, "./testdata/PreInfoContainer.json")
	result2 := testutils.LoadContainerInfo(t, "./testdata/CurInfoContainer.json")

	// test container type
	containerType := containerinsight.TypeContainer
	extractor := NewCPUMetricExtractor(nil)

	var cMetrics []*CAdvisorMetric
	if extractor.HasValue(result[0]) {
		cMetrics = extractor.GetValue(result[0], MockCPUMemInfo, containerType)
	}

	if extractor.HasValue(result2[0]) {
		cMetrics = extractor.GetValue(result2[0], MockCPUMemInfo, containerType)
	}

	AssertContainsTaggedFloat(t, cMetrics[0], "container_cpu_usage_total", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "container_cpu_usage_user", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "container_cpu_usage_system", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "container_cpu_utilization", 0.5, 0)

	// test node type
	containerType = containerinsight.TypeNode
	require.NoError(t, extractor.Shutdown())
	extractor = NewCPUMetricExtractor(nil)

	if extractor.HasValue(result[0]) {
		cMetrics = extractor.GetValue(result[0], MockCPUMemInfo, containerType)
	}

	if extractor.HasValue(result2[0]) {
		cMetrics = extractor.GetValue(result2[0], MockCPUMemInfo, containerType)
	}

	AssertContainsTaggedFloat(t, cMetrics[0], "node_cpu_usage_total", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "node_cpu_usage_user", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "node_cpu_usage_system", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "node_cpu_utilization", 0.5, 0)
	AssertContainsTaggedInt(t, cMetrics[0], "node_cpu_limit", 2000)

	// test instance type
	containerType = containerinsight.TypeInstance
	require.NoError(t, extractor.Shutdown())
	extractor = NewCPUMetricExtractor(nil)

	if extractor.HasValue(result[0]) {
		cMetrics = extractor.GetValue(result[0], MockCPUMemInfo, containerType)
	}

	if extractor.HasValue(result2[0]) {
		cMetrics = extractor.GetValue(result2[0], MockCPUMemInfo, containerType)
	}

	AssertContainsTaggedFloat(t, cMetrics[0], "instance_cpu_usage_total", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "instance_cpu_usage_user", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "instance_cpu_usage_system", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "instance_cpu_utilization", 0.5, 0)
	AssertContainsTaggedInt(t, cMetrics[0], "instance_cpu_limit", 2000)
	require.NoError(t, extractor.Shutdown())
}
