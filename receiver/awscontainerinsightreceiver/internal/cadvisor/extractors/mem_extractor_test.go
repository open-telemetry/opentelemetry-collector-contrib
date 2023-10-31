// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"
)

func TestMemStats(t *testing.T) {
	MockCPUMemInfo := testutils.MockCPUMemInfo{}
	result := testutils.LoadContainerInfo(t, "./testdata/PreInfoContainer.json")
	result2 := testutils.LoadContainerInfo(t, "./testdata/CurInfoContainer.json")

	containerType := containerinsight.TypeContainer
	extractor := NewMemMetricExtractor(nil)

	var cMetrics []*CAdvisorMetric
	if extractor.HasValue(result[0]) {
		cMetrics = extractor.GetValue(result[0], MockCPUMemInfo, containerType)
	}

	if extractor.HasValue(result2[0]) {
		cMetrics = extractor.GetValue(result2[0], MockCPUMemInfo, containerType)
	}

	AssertContainsTaggedUint(t, cMetrics[0], "container_memory_cache", 25645056)
	AssertContainsTaggedUint(t, cMetrics[0], "container_memory_rss", 221184)
	AssertContainsTaggedUint(t, cMetrics[0], "container_memory_max_usage", 90775552)
	AssertContainsTaggedUint(t, cMetrics[0], "container_memory_mapped_file", 0)
	AssertContainsTaggedUint(t, cMetrics[0], "container_memory_usage", 29728768)
	AssertContainsTaggedUint(t, cMetrics[0], "container_memory_swap", 0)
	AssertContainsTaggedUint(t, cMetrics[0], "container_memory_failcnt", 0)
	AssertContainsTaggedUint(t, cMetrics[0], "container_memory_working_set", 28844032)

	AssertContainsTaggedFloat(t, cMetrics[0], "container_memory_pgfault", 1000, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "container_memory_hierarchical_pgfault", 1000, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "container_memory_pgmajfault", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "container_memory_hierarchical_pgmajfault", 10, 0)

	// for node type
	containerType = containerinsight.TypeNode
	require.NoError(t, extractor.Shutdown())
	extractor = NewMemMetricExtractor(nil)

	if extractor.HasValue(result[0]) {
		cMetrics = extractor.GetValue(result[0], MockCPUMemInfo, containerType)
	}

	if extractor.HasValue(result2[0]) {
		cMetrics = extractor.GetValue(result2[0], MockCPUMemInfo, containerType)
	}

	AssertContainsTaggedUint(t, cMetrics[0], "node_memory_cache", 25645056)
	AssertContainsTaggedUint(t, cMetrics[0], "node_memory_rss", 221184)
	AssertContainsTaggedUint(t, cMetrics[0], "node_memory_max_usage", 90775552)
	AssertContainsTaggedUint(t, cMetrics[0], "node_memory_mapped_file", 0)
	AssertContainsTaggedUint(t, cMetrics[0], "node_memory_usage", 29728768)
	AssertContainsTaggedUint(t, cMetrics[0], "node_memory_swap", 0)
	AssertContainsTaggedUint(t, cMetrics[0], "node_memory_failcnt", 0)
	AssertContainsTaggedUint(t, cMetrics[0], "node_memory_working_set", 28844032)
	AssertContainsTaggedInt(t, cMetrics[0], "node_memory_limit", 1073741824)

	AssertContainsTaggedFloat(t, cMetrics[0], "node_memory_pgfault", 1000, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "node_memory_hierarchical_pgfault", 1000, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "node_memory_pgmajfault", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "node_memory_hierarchical_pgmajfault", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "node_memory_utilization", 2.68630981, 1.0e-8)

	// for instance type
	containerType = containerinsight.TypeInstance
	require.NoError(t, extractor.Shutdown())
	extractor = NewMemMetricExtractor(nil)

	if extractor.HasValue(result[0]) {
		cMetrics = extractor.GetValue(result[0], MockCPUMemInfo, containerType)
	}

	if extractor.HasValue(result2[0]) {
		cMetrics = extractor.GetValue(result2[0], MockCPUMemInfo, containerType)
	}

	AssertContainsTaggedUint(t, cMetrics[0], "instance_memory_cache", 25645056)
	AssertContainsTaggedUint(t, cMetrics[0], "instance_memory_rss", 221184)
	AssertContainsTaggedUint(t, cMetrics[0], "instance_memory_max_usage", 90775552)
	AssertContainsTaggedUint(t, cMetrics[0], "instance_memory_mapped_file", 0)
	AssertContainsTaggedUint(t, cMetrics[0], "instance_memory_usage", 29728768)
	AssertContainsTaggedUint(t, cMetrics[0], "instance_memory_swap", 0)
	AssertContainsTaggedUint(t, cMetrics[0], "instance_memory_failcnt", 0)
	AssertContainsTaggedUint(t, cMetrics[0], "instance_memory_working_set", 28844032)
	AssertContainsTaggedInt(t, cMetrics[0], "instance_memory_limit", 1073741824)

	AssertContainsTaggedFloat(t, cMetrics[0], "instance_memory_pgfault", 1000, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "instance_memory_hierarchical_pgfault", 1000, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "instance_memory_pgmajfault", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "instance_memory_hierarchical_pgmajfault", 10, 0)
	AssertContainsTaggedFloat(t, cMetrics[0], "instance_memory_utilization", 2.68630981, 1.0e-8)
	require.NoError(t, extractor.Shutdown())
}
