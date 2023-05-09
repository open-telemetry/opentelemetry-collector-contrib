// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package converter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.8.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/backend"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"
)

var _ Converter = (*SpanConverter)(nil)

type SpanConverter struct {
	logger *zap.Logger
}

func (c *SpanConverter) AcceptsSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) bool {

	return true
}

func (c *SpanConverter) ConvertSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) model.Bundle {
	bundle := model.NewBundle()
	fromS := model.FromS{}

	hostIDValue, ex := attributes.Get(backend.AttributeInstanaHostID)
	if !ex {
		fromS.HostID = "unknown-host-id"
	} else {
		fromS.HostID = hostIDValue.AsString()
	}

	processIDValue, ex := attributes.Get(conventions.AttributeProcessPID)
	if !ex {
		fromS.EntityID = "unknown-process-id"
	} else {
		fromS.EntityID = processIDValue.AsString()
	}

	serviceName := ""
	serviceNameValue, ex := attributes.Get(conventions.AttributeServiceName)
	if ex {
		serviceName = serviceNameValue.AsString()
	}

	for i := 0; i < spanSlice.Len(); i++ {
		otelSpan := spanSlice.At(i)

		instanaSpan, err := model.ConvertPDataSpanToInstanaSpan(fromS, otelSpan, serviceName, attributes)
		if err != nil {
			c.logger.Warn(fmt.Sprintf("Error converting Open Telemetry span to Instana span: %s", err.Error()))
			continue
		}

		bundle.Spans = append(bundle.Spans, instanaSpan)
	}

	return bundle
}

func (c *SpanConverter) Name() string {
	return "SpanConverter"
}
