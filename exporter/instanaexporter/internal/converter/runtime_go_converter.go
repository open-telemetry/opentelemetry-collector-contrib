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

var _ Converter = (*RuntimeGoConverter)(nil)

type RuntimeGoConverter struct{}

func (c *RuntimeGoConverter) AcceptsMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) bool {
	runtimeAttr, ex := attributes.Get(conventions.AttributeTelemetrySDKLanguage)

	return ex && runtimeAttr.AsString() == conventions.AttributeTelemetrySDKLanguageGo
}

func (c *RuntimeGoConverter) ConvertMetrics(attributes pcommon.Map, metricSlice pmetric.MetricSlice) []instanaacceptor.PluginPayload {
	processPidAttr, ex := attributes.Get(conventions.AttributeProcessPID)
	if !ex {
		return make([]instanaacceptor.PluginPayload, 0)
	}

	return []instanaacceptor.PluginPayload{
		instanaacceptor.NewGoProcessPluginPayload(
			createGoSnapshot(attributes, int(processPidAttr.Int())),
		),
	}
}

func (c *RuntimeGoConverter) AcceptsSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) bool {

	runtimeAttr, ex := attributes.Get(conventions.AttributeTelemetrySDKLanguage)

	return ex && runtimeAttr.AsString() == conventions.AttributeTelemetrySDKLanguageGo
}

func (c *RuntimeGoConverter) ConvertSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) model.Bundle {
	bundle := model.NewBundle()
	processPidAttr, ex := attributes.Get(conventions.AttributeProcessPID)
	if !ex {
		return bundle
	}

	bundle.Metrics.Plugins = append(bundle.Metrics.Plugins, instanaacceptor.NewGoProcessPluginPayload(
		createGoSnapshot(attributes, int(processPidAttr.Int())),
	))

	return bundle
}

func (c *RuntimeGoConverter) Name() string {
	return "RuntimeGoConverter"
}

func createGoSnapshot(attributes pcommon.Map, processPid int) instanaacceptor.GoProcessData {
	runtimeInfo := instanaacceptor.RuntimeInfo{}

	processNameAttr, ex := attributes.Get(conventions.AttributeProcessExecutableName)
	if ex {
		runtimeInfo.Name = processNameAttr.AsString()
	}

	runtimeNameAttr, ex := attributes.Get(conventions.AttributeProcessRuntimeName)
	if ex {
		runtimeInfo.Compiler = runtimeNameAttr.AsString()
	}

	runtimeVersionAttr, ex := attributes.Get(conventions.AttributeProcessRuntimeVersion)
	if ex {
		runtimeInfo.Version = runtimeVersionAttr.AsString()
	}

	return instanaacceptor.GoProcessData{
		PID:      processPid,
		Snapshot: &runtimeInfo,
	}
}
