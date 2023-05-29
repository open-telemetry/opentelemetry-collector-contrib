// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

func (c *SpanConverter) AcceptsSpans(_ pcommon.Map, _ ptrace.SpanSlice) bool {
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
