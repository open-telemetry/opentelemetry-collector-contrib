// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traceutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestSpanKindStr(t *testing.T) {
	assert.EqualValues(t, "SPAN_KIND_UNSPECIFIED", SpanKindStr(ptrace.SpanKindUnspecified))
	assert.EqualValues(t, "SPAN_KIND_INTERNAL", SpanKindStr(ptrace.SpanKindInternal))
	assert.EqualValues(t, "SPAN_KIND_SERVER", SpanKindStr(ptrace.SpanKindServer))
	assert.EqualValues(t, "SPAN_KIND_CLIENT", SpanKindStr(ptrace.SpanKindClient))
	assert.EqualValues(t, "SPAN_KIND_PRODUCER", SpanKindStr(ptrace.SpanKindProducer))
	assert.EqualValues(t, "SPAN_KIND_CONSUMER", SpanKindStr(ptrace.SpanKindConsumer))
	assert.EqualValues(t, "", SpanKindStr(ptrace.SpanKind(100)))
}

func TestStatusCodeStr(t *testing.T) {
	assert.EqualValues(t, "STATUS_CODE_UNSET", StatusCodeStr(ptrace.StatusCodeUnset))
	assert.EqualValues(t, "STATUS_CODE_OK", StatusCodeStr(ptrace.StatusCodeOk))
	assert.EqualValues(t, "STATUS_CODE_ERROR", StatusCodeStr(ptrace.StatusCodeError))
	assert.EqualValues(t, "", StatusCodeStr(ptrace.StatusCode(100)))
}
