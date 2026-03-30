// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package url

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestShouldSanitizeSpanRequiresHTTPSignal(t *testing.T) {
	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindClient)
	span.SetName("operation_without_route")

	assert.False(t, shouldSanitizeSpan(span))

	span.Attributes().PutStr("http.route", "/users/{id}")
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

func TestSanitizeSpanNameNilSanitizer(t *testing.T) {
	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindClient)
	span.SetName("/users/123")

	sanitized, ok := SanitizeSpanName(span, nil)
	assert.False(t, ok)
	assert.Empty(t, sanitized)
}

func TestSanitizeSpanNameInternalSpan(t *testing.T) {
	sanitizer, err := NewURLSanitizer(URLSanitizationConfig{})
	require.NoError(t, err)

	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindInternal)
	span.SetName("/users/123")

	sanitized, ok := SanitizeSpanName(span, sanitizer)
	assert.False(t, ok)
	assert.Empty(t, sanitized)
}

func TestSanitizeSpanNameNoHTTPAttributesNoSlash(t *testing.T) {
	sanitizer, err := NewURLSanitizer(URLSanitizationConfig{})
	require.NoError(t, err)

	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindClient)
	span.SetName("operation-name")

	sanitized, ok := SanitizeSpanName(span, sanitizer)
	assert.False(t, ok)
	assert.Empty(t, sanitized)
}

func TestSanitizeSpanNameWithHTTPAttributesNoSlash(t *testing.T) {
	sanitizer, err := NewURLSanitizer(URLSanitizationConfig{})
	require.NoError(t, err)

	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindServer)
	span.SetName("operation-123")
	span.Attributes().PutStr("http.method", "GET")

	sanitized, ok := SanitizeSpanName(span, sanitizer)
	// The span name was fully redacted to *, so we return original name with ok=false
	assert.False(t, ok)
	assert.Equal(t, "operation-123", sanitized)
}

func TestSanitizeSpanNameUnchanged(t *testing.T) {
	sanitizer, err := NewURLSanitizer(URLSanitizationConfig{})
	require.NoError(t, err)

	span := ptrace.NewSpan()
	span.SetKind(ptrace.SpanKindClient)
	span.SetName("/users/profile")

	sanitized, ok := SanitizeSpanName(span, sanitizer)
	assert.False(t, ok)
	assert.Empty(t, sanitized)
}

func TestSanitizeSpanNameFullyRedacted(t *testing.T) {
	tests := []struct {
		name         string
		spanName     string
		expectedName string
		expectedOk   bool
		description  string
	}{
		{
			name:         "single numeric segment becomes asterisk - keep original",
			spanName:     "/123",
			expectedName: "/123",
			expectedOk:   false,
			description:  "Fully numeric path sanitizes to /* which should not replace span name",
		},
		{
			name:         "two numeric segments become asterisks - keep original",
			spanName:     "/123/456",
			expectedName: "/123/456",
			expectedOk:   false,
			description:  "Fully numeric path sanitizes to /*/* which should not replace span name",
		},
		{
			name:         "three numeric segments become asterisks - keep original",
			spanName:     "/123/456/789",
			expectedName: "/123/456/789",
			expectedOk:   false,
			description:  "Fully numeric path sanitizes to */*/* which should not replace span name",
		},
		{
			name:         "partial redaction is applied",
			spanName:     "/payments/12345",
			expectedName: "/payments/*",
			expectedOk:   true,
			description:  "Mixed path with literal and numeric segment is partially redacted",
		},
		{
			name:         "mixed pattern with non-asterisk segment is applied",
			spanName:     "/payments/12345/detail",
			expectedName: "/payments/*/detail",
			expectedOk:   true,
			description:  "Path with literals keeps them and redacts only numeric parts",
		},
		{
			name:         "unchanged name returns false",
			spanName:     "/payments/users",
			expectedName: "",
			expectedOk:   false,
			description:  "Path with no redactable content returns unchanged",
		},
		{
			name:         "single slash is valid literal",
			spanName:     "/",
			expectedName: "",
			expectedOk:   false,
			description:  "Single slash is a valid span name and should not be treated as fully redacted",
		},
	}

	sanitizer, err := NewURLSanitizer(URLSanitizationConfig{})
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			span.SetKind(ptrace.SpanKindClient)
			span.SetName(tt.spanName)

			sanitized, ok := SanitizeSpanName(span, sanitizer)
			assert.Equal(t, tt.expectedOk, ok, tt.description)
			assert.Equal(t, tt.expectedName, sanitized, tt.description)
		})
	}
}

func TestWasFullyRedacted(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "single asterisk",
			input:    "*",
			expected: true,
		},
		{
			name:     "two segment pattern",
			input:    "/*/*",
			expected: true,
		},
		{
			name:     "three segment pattern",
			input:    "/*/*/*",
			expected: true,
		},
		{
			name:     "four segment pattern",
			input:    "/*/*/*/*",
			expected: true,
		},
		{
			name:     "pattern with trailing slash",
			input:    "/*/*/",
			expected: true,
		},
		{
			name:     "pattern with leading and trailing slashes",
			input:    "/*/*/",
			expected: true,
		},
		{
			name:     "partial pattern with literal segment",
			input:    "/*/detail",
			expected: false,
		},
		{
			name:     "partial pattern with literal at start",
			input:    "/users/*",
			expected: false,
		},
		{
			name:     "partial pattern in middle",
			input:    "/users/*/detail",
			expected: false,
		},
		{
			name:     "literal path",
			input:    "/users/123",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "just slashes",
			input:    "///",
			expected: false,
		},
		{
			name:     "single slash",
			input:    "/",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wasFullyRedacted(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
