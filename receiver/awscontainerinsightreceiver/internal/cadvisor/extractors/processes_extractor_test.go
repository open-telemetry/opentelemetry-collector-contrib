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

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	. "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"
)

func TestProcessesStats(t *testing.T) {
	result := testutils.LoadContainerInfo(t, "./testdata/CurInfoContainer.json")
	extractor := NewProcessesMetricExtractor(zap.NewNop())

	var cMetrics []*CAdvisorMetric

	if !extractor.HasValue(result[0]) {
		t.Logf("Testdata not configured to work with extractor")
		t.FailNow()
	}

	// Type Container
	cMetrics = extractor.GetValue(result[0], nil, TypeContainer)
	expectedFields := map[string]interface{}{
		"container_processes":                  uint64(10),
		"container_processes_threads":          uint64(20),
		"container_processes_file_descriptors": uint64(30),
	}
	expectedTags := map[string]string{
		"Type": "Container",
	}
	AssertContainsTaggedField(t, cMetrics[0], expectedFields, expectedTags)

	// Type Pod
	cMetrics = extractor.GetValue(result[0], nil, TypePod)
	assert.Equal(t, len(cMetrics), 0)
}
