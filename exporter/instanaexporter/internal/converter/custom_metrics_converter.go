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
	instanaacceptor "github.com/instana/go-sensor/acceptor"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.8.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"
)

var _ Converter = (*CustomMetricsConverter)(nil)

type CustomMetricsConverter struct{}

func (c *CustomMetricsConverter) AcceptsMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) bool {
	return true
}

func (c *CustomMetricsConverter) ConvertMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) []instanaacceptor.PluginPayload {
	// return early if no metrics contained
	if metricSlice.Len() == 0 {
		return make([]instanaacceptor.PluginPayload, 0)
	}

	entityID := "h"
	if pidAttribute, ex := attributes.Get(conventions.AttributeProcessPID); ex {
		entityID = pidAttribute.AsString()
	}

	metricData := model.NewOpenTelemetryCustomMetricsData()

	for i := 0; i < metricSlice.Len(); i++ {
		metric := metricSlice.At(i)

		metricData.AppendMetric(metric)
	}

	metricsPayload := model.NewOpenTelemetryMetricsPluginPayload(entityID, metricData)

	return []instanaacceptor.PluginPayload{
		metricsPayload,
	}
}

func (c *CustomMetricsConverter) AcceptsSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) bool {

	return false
}

func (c *CustomMetricsConverter) ConvertSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) model.Bundle {

	return model.NewBundle()
}

func (c *CustomMetricsConverter) Name() string {
	return "CustomMetricsConverter"
}
