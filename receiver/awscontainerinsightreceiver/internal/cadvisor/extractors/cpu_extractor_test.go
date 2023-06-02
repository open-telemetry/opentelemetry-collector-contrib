// Copyright The OpenTelemetry Authors
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

func TestCPUStats(t *testing.T) {
	MockCPUMemInfo := testutils.MockCPUMemInfo{}

	result := testutils.LoadContainerInfo(t, "./testdata/PreInfoContainer.json")
	result2 := testutils.LoadContainerInfo(t, "./testdata/CurInfoContainer.json")

	// test container type
	containerType := TypeContainer
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
	containerType = TypeNode
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
	containerType = TypeInstance
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
}
