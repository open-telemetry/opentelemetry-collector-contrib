// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traceutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestSpanKindStr(t *testing.T) {
	assert.Equal(t, "SPAN_KIND_UNSPECIFIED", SpanKindStr(ptrace.SpanKindUnspecified))
	assert.Equal(t, "SPAN_KIND_INTERNAL", SpanKindStr(ptrace.SpanKindInternal))
	assert.Equal(t, "SPAN_KIND_SERVER", SpanKindStr(ptrace.SpanKindServer))
	assert.Equal(t, "SPAN_KIND_CLIENT", SpanKindStr(ptrace.SpanKindClient))
	assert.Equal(t, "SPAN_KIND_PRODUCER", SpanKindStr(ptrace.SpanKindProducer))
	assert.Equal(t, "SPAN_KIND_CONSUMER", SpanKindStr(ptrace.SpanKindConsumer))
	assert.Empty(t, SpanKindStr(ptrace.SpanKind(100)))
}

func TestStatusCodeStr(t *testing.T) {
	assert.Equal(t, "STATUS_CODE_UNSET", StatusCodeStr(ptrace.StatusCodeUnset))
	assert.Equal(t, "STATUS_CODE_OK", StatusCodeStr(ptrace.StatusCodeOk))
	assert.Equal(t, "STATUS_CODE_ERROR", StatusCodeStr(ptrace.StatusCodeError))
	assert.Empty(t, StatusCodeStr(ptrace.StatusCode(100)))
}
