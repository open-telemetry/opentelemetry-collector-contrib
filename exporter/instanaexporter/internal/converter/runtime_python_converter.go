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

var _ Converter = (*RuntimePythonConverter)(nil)

type RuntimePythonConverter struct{}

func (c *RuntimePythonConverter) AcceptsMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) bool {
	runtimeAttr, ex := attributes.Get(conventions.AttributeTelemetrySDKLanguage)

	return ex && runtimeAttr.AsString() == conventions.AttributeTelemetrySDKLanguagePython
}

func (c *RuntimePythonConverter) ConvertMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) []instanaacceptor.PluginPayload {
	processPidAttr, ex := attributes.Get(conventions.AttributeProcessPID)
	if !ex {
		return make([]instanaacceptor.PluginPayload, 0)
	}

	return []instanaacceptor.PluginPayload{
		model.NewPythonRuntimePlugin(
			createPythonSnapshot(attributes, int(processPidAttr.Int())),
		),
	}
}

func (c *RuntimePythonConverter) AcceptsSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) bool {

	runtimeAttr, ex := attributes.Get(conventions.AttributeTelemetrySDKLanguage)

	return ex && runtimeAttr.AsString() == conventions.AttributeTelemetrySDKLanguagePython
}

func (c *RuntimePythonConverter) ConvertSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) model.Bundle {
	bundle := model.NewBundle()
	processPidAttr, ex := attributes.Get(conventions.AttributeProcessPID)
	if !ex {
		return bundle
	}

	bundle.Metrics.Plugins = append(bundle.Metrics.Plugins, model.NewPythonRuntimePlugin(
		createPythonSnapshot(attributes, int(processPidAttr.Int())),
	))

	return bundle
}

func (c *RuntimePythonConverter) Name() string {
	return "RuntimePythonConverter"
}

func createPythonSnapshot(attributes pcommon.Map, processPid int) model.PythonProcessData {
	processData := model.PythonProcessData{
		PID: processPid,
	}

	processNameAttr, ex := attributes.Get(conventions.AttributeProcessExecutableName)
	if ex {
		processData.Name = processNameAttr.AsString()
	}

	runtimeNameAttr, ex := attributes.Get(conventions.AttributeProcessRuntimeName)
	if ex {
		processData.PythonFlavor = runtimeNameAttr.AsString()
	}

	runtimeVersionAttr, ex := attributes.Get(conventions.AttributeProcessRuntimeVersion)
	if ex {
		processData.PythonVersion = runtimeVersionAttr.AsString()
	}

	return processData
}
