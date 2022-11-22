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
	"os"
	"strconv"

	instanaacceptor "github.com/instana/go-sensor/acceptor"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"
)

var _ Converter = (*CollectorMetricsConverter)(nil)

type CollectorMetricsConverter struct{}

func (c *CollectorMetricsConverter) AcceptsMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) bool {

	return containsMetricWithPrefix(metricSlice, "otelcol_")
}

func (c *CollectorMetricsConverter) ConvertMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) []instanaacceptor.PluginPayload {
	pid := os.Getpid()

	collectorProcessPlugin := instanaacceptor.NewProcessPluginPayload(strconv.Itoa(pid), instanaacceptor.ProcessData{
		PID:     pid,
		Exec:    "otelcol-idot",
		HostPID: pid,
	})

	goPlugin := instanaacceptor.NewGoProcessPluginPayload(instanaacceptor.GoProcessData{
		PID: pid,
		Snapshot: &instanaacceptor.RuntimeInfo{
			Name: "OpenTelemetry Collector",
		},
	})

	customMetricsData := model.NewOpenTelemetryCustomMetricsData()
	customMetricsData.Pid = strconv.Itoa(pid)

	for i := 0; i < metricSlice.Len(); i++ {
		metric := metricSlice.At(i)

		customMetricsData.AppendMetric(metric)
	}

	otelMetricsCollectorPlugin := model.NewOpenTelemetryMetricsPluginPayload(strconv.Itoa(pid), customMetricsData)

	return []instanaacceptor.PluginPayload{
		collectorProcessPlugin,
		goPlugin,
		otelMetricsCollectorPlugin,
	}
}

func (c *CollectorMetricsConverter) AcceptsSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) bool {

	return false
}

func (c *CollectorMetricsConverter) ConvertSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) model.Bundle {

	return model.NewBundle()
}

func (c *CollectorMetricsConverter) Name() string {
	return "CollectorMetricsConverter"
}
