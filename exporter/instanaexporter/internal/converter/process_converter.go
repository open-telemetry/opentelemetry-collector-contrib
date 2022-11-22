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
	"strings"

	instanaacceptor "github.com/instana/go-sensor/acceptor"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.8.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"
)

var _ Converter = (*ProcessMetricConverter)(nil)

type ProcessMetricConverter struct{}

func (c *ProcessMetricConverter) AcceptsMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) bool {
	_, ex := attributes.Get(conventions.AttributeProcessPID)

	return ex
}

func (c *ProcessMetricConverter) ConvertMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) []instanaacceptor.PluginPayload {
	plugins := make([]instanaacceptor.PluginPayload, 0)

	processPidAttr, ex := attributes.Get(conventions.AttributeProcessPID)
	if !ex {
		return plugins
	}

	processData := createProcessData(attributes, int(processPidAttr.Int()))

	plugins = append(plugins, instanaacceptor.NewProcessPluginPayload(processPidAttr.AsString(), processData))

	return plugins
}

func (c *ProcessMetricConverter) AcceptsSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) bool {
	_, ex := attributes.Get(conventions.AttributeProcessPID)

	return ex
}

func (c *ProcessMetricConverter) ConvertSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) model.Bundle {

	bundle := model.NewBundle()

	processPidAttr, ex := attributes.Get(conventions.AttributeProcessPID)
	if !ex {
		return bundle
	}

	processData := createProcessData(attributes, int(processPidAttr.Int()))
	bundle.Metrics.Plugins = append(bundle.Metrics.Plugins, instanaacceptor.NewProcessPluginPayload(processPidAttr.AsString(), processData))

	return bundle
}

func (c *ProcessMetricConverter) Name() string {
	return "ProcessMetricConverter"
}

func createProcessData(attributes pcommon.Map, processPid int) instanaacceptor.ProcessData {
	processData := instanaacceptor.ProcessData{
		PID: processPid,
	}

	processExecAttr, ex := attributes.Get(conventions.AttributeProcessExecutablePath)
	if ex {
		processData.Exec = processExecAttr.AsString()
	}

	processCommmandArgsAttr, ex := attributes.Get(conventions.AttributeProcessCommandArgs)
	if ex {
		processData.Args = strings.Split(processCommmandArgsAttr.AsString(), ", ")
	}

	// container info
	processContainerIDAttr, ex := attributes.Get(conventions.AttributeContainerID)
	if ex {
		processData.ContainerID = processContainerIDAttr.AsString()
	}

	return processData
}
