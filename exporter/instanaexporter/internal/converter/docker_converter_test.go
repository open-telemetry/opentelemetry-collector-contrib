// Copyright  The OpenTelemetry Authors
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

package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.8.0"
)

func TestCanConvert(t *testing.T) {
	converter := DockerContainerMetricConverter{}

	attributes := pcommon.NewMap()
	attributes.PutStr(conventions.AttributeContainerRuntime, "docker")
	attributes.PutStr(conventions.AttributeContainerID, "abc")
	attributes.PutStr(conventions.AttributeContainerImageName, "ubuntu")
	attributes.PutStr(conventions.AttributeContainerImageTag, "latest")
	attributes.PutStr(conventions.AttributeContainerName, "my-container")

	metrics := pmetric.NewMetricSlice()
	metric := metrics.AppendEmpty()
	metric.SetName("container.network.io.usage.tx_packets")
	metric.SetDescription("")
	metric.SetUnit("1")
	metric.SetEmptySum()
	metric.Sum().SetIsMonotonic(true)
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	md := metric.Sum().DataPoints().AppendEmpty()
	md.SetIntValue(0)
	md.Attributes().PutStr("interface", "eth0")

	plugins := converter.ConvertMetrics(attributes, metrics)

	assert.Equal(t, 1, len(plugins))
	assert.Equal(t, "com.instana.plugin.docker", plugins[0].Name)
}
