// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package converter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/instanaexporter/internal/converter/model"
)

type Converter interface {
	AcceptsSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) bool
	ConvertSpans(attributes pcommon.Map, spanSlice ptrace.SpanSlice) model.Bundle
	Name() string
}
