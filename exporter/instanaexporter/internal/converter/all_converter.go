// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package converter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"
)

var _ Converter = (*ConvertAllConverter)(nil)

type ConvertAllConverter struct {
	converters []Converter
	logger     *zap.Logger
}

func (c *ConvertAllConverter) AcceptsSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) bool {
	return true
}

func (c *ConvertAllConverter) ConvertSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) model.Bundle {
	bundle := model.NewBundle()

	for i := 0; i < len(c.converters); i++ {
		if !c.converters[i].AcceptsSpans(attributes, spanSlice) {
			c.logger.Warn(fmt.Sprintf("Converter %q didn't accept spans", c.converters[i].Name()))

			continue
		}

		converterBundle := c.converters[i].ConvertSpans(attributes, spanSlice)
		if len(converterBundle.Spans) > 0 {
			bundle.Spans = append(bundle.Spans, converterBundle.Spans...)
		}
	}

	return bundle
}

func (c *ConvertAllConverter) Name() string {
	return "ConvertAllConverter"
}

func NewConvertAllConverter(logger *zap.Logger) Converter {

	return &ConvertAllConverter{
		converters: []Converter{
			&SpanConverter{logger: logger},
		},
		logger: logger,
	}
}
