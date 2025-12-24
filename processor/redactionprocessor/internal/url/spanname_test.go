// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package url

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv128 "go.opentelemetry.io/otel/semconv/v1.28.0"
)

func TestShouldSanitizeSpanRequiresHTTPSignal(t *testing.T) {
	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindClient)
	span.SetName("operation_without_route")

	assert.False(t, shouldSanitizeSpan(span))

	span.Attributes().PutStr(string(semconv128.HTTPRouteKey), "/users/{id}")
	assert.True(t, shouldSanitizeSpan(span))
}

func TestShouldSanitizeSpanRequiresClientOrServer(t *testing.T) {
	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindInternal)
	span.SetName("/users/123")

	assert.False(t, shouldSanitizeSpan(span))
}

func TestSanitizeSpanName(t *testing.T) {
	sanitizer, err := NewURLSanitizer(URLSanitizationConfig{})
	require.NoError(t, err)

	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindClient)
	span.SetName("/payments/12345/detail")

	sanitized, ok := SanitizeSpanName(span, sanitizer)
	assert.True(t, ok)
	assert.Equal(t, "/payments/*/detail", sanitized)
}
