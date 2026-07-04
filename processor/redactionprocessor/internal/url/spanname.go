// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package url // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/url"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// SanitizeSpanName sanitizes the span name if the span looks like an HTTP span.
// It returns the sanitized name and true when a change was made.
func SanitizeSpanName(span ptrace.Span, sanitizer *URLSanitizer) (string, bool) {
	if sanitizer == nil {
		return "", false
	}

	if !shouldSanitizeSpan(span) {
		return "", false
	}

	name := span.Name()
	sanitized := sanitizer.SanitizeURL(name)
	if sanitized == name {
		return "", false
	}

	// This means the full span name was replaced
	if wasFullyRedacted(sanitized) {
		return name, false
	}

	return sanitized, true
}

func wasFullyRedacted(s string) bool {
	if s == "" {
		return false
	}
	segments := strings.Split(s, "/")
	hasAsterisk := false
	for _, segment := range segments {
		if segment == "*" {
			hasAsterisk = true
		} else if segment != "" {
			return false
		}
	}
	return hasAsterisk
}

func shouldSanitizeSpan(span ptrace.Span) bool {
	kind := span.Kind()
	if kind != ptrace.SpanKindClient && kind != ptrace.SpanKindServer {
		return false
	}

	attrs := span.Attributes()
	spanName := span.Name()

	if !hasHTTPAttributes(attrs) && !strings.Contains(spanName, "/") {
		return false
	}

	return true
}

var httpAttributeKeys = []string{
	string(conventions.HTTPRouteKey),
	string(conventions.HTTPRequestMethodKey),
	string(conventions.HTTPRequestMethodOriginalKey),
	string(conventions.HTTPResponseStatusCodeKey),
	string(conventions.URLFullKey),
	"http.scheme",
	"http.target",
	"http.method",
	"http.url",
}

func hasHTTPAttributes(attrs pcommon.Map) bool {
	for _, key := range httpAttributeKeys {
		if _, ok := attrs.Get(key); ok {
			return true
		}
	}
	return false
}
