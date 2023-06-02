// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package extractors

import (
	"testing"

	. "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"
)

func TestMemStats(t *testing.T) {
	MockCPUMemInfo := testutils.MockCPUMemInfo{}
	result := testutils.LoadContainerInfo(t, "./testdata/PreInfoContainer.json")
	result2 := testutils.LoadContainerInfo(t, "./testdata/CurInfoContainer.json")

	containerType := TypeContainer
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
	AssertContainsTaggedFloat(t, cMetrics[0], "container_memory_failures_total", 1010, 0.0000001)

	// for node type
	containerType = TypeNode
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
	containerType = TypeInstance
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

}
