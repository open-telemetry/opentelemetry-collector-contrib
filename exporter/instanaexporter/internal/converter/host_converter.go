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
	"fmt"
	"math"
	"regexp"
	"strconv"

	instanaacceptor "github.com/instana/go-sensor/acceptor"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.8.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"
)

var _ Converter = (*HostMetricConverter)(nil)

type HostMetricConverter struct{}

func (c *HostMetricConverter) AcceptsMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) bool {
	return containsMetricWithPrefix(metricSlice, "system.")
}

// func (c *HostMetricConverter) ConvertMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) (pluginPayLoad []instanaacceptor.PluginPayload, retErr error) {
func (c *HostMetricConverter) ConvertMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) []instanaacceptor.PluginPayload {
	/*defer func() {
		if r := recover(); r != nil {
			reterr = fmt.Errorf("unable to parse expression: %s", r)
		}
	}()*/

	hostData := model.NewHostData()

	attributes.Range(func(name string, value pcommon.Value) bool {
		hostData.AddTag(fmt.Sprintf("%s=%s", name, value.AsString()))

		return true
	})

	hostNameAttribute, ex := attributes.Get(conventions.AttributeHostName)
	if ex {
		hostData.HostName = hostNameAttribute.AsString()
	}

	osTypeAttribute, ex := attributes.Get(conventions.AttributeOSType)
	if ex {
		hostData.OsName = osTypeAttribute.AsString()
	}

	cpuCount := 0
	cpuSummaries := make([]model.CPUSummary, 0)

	// gather CPU data
	for i := 0; i < metricSlice.Len(); i++ {
		metric := metricSlice.At(i)

		r := regexp.MustCompile(`[0-9]+`)

		if metric.Name() == "system.cpu.time" {
			for j := 0; j < metric.Sum().DataPoints().Len(); j++ {
				dp := metric.Sum().DataPoints().At(j)

				var cpuNo string
				cpuAttribute, ex := dp.Attributes().Get("cpu")
				if ex {
					cpuNo = r.FindString(cpuAttribute.AsString())
				} else {
					// is see if we can make extraction more simple
					continue
				}

				cpuNoInt, err := strconv.Atoi(cpuNo)
				if err != nil {
					panic(err)
				}

				if len(cpuSummaries) <= cpuNoInt+1 {
					cpuSummaries = append(cpuSummaries, model.CPUSummary{})
				}

				stateAttribute, ex := dp.Attributes().Get("state")
				if ex && stateAttribute.AsString() == "system" {
					cpuCount++
				}

				switch stateAttribute.AsString() {
				case "idle":
					cpuSummaries[cpuNoInt].Idle = math.Round(dp.DoubleValue()*100) / 100000000
				case "interrupt":
					cpuSummaries[cpuNoInt].Steal = math.Round(dp.DoubleValue()*100) / 100000000
				case "system":
					cpuSummaries[cpuNoInt].Sys = math.Round(dp.DoubleValue()*100) / 100000000
				case "user":
					cpuSummaries[cpuNoInt].User = math.Round(dp.DoubleValue()*100) / 100000000
					// TODO: Add "nice" DataPoint in hostmetricsreceiver
					// case "user":
					//	hostData.AddFloatMetric(fmt.Sprintf("cpus.%s.%s", cpuNo, "user"), dp.DoubleVal())
				}
			}

			if len(cpuSummaries) > 0 {
				hostData.CPU = cpuSummaries[0]
				hostData.CPUSummaries = append(hostData.CPUSummaries, cpuSummaries[1:]...)
			}
		}

		hostData.CPUCount = cpuCount
	}

	return []instanaacceptor.PluginPayload{
		model.NewHostPluginPayload("h", hostData),
	}
}

func (c *HostMetricConverter) AcceptsSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) bool {

	return false
}

func (c *HostMetricConverter) ConvertSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) model.Bundle {

	return model.NewBundle()
}

func (c *HostMetricConverter) Name() string {
	return "HostMetricConverter"
}
