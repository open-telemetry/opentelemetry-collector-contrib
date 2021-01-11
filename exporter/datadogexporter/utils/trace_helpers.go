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

package utils

import (
	"strings"
	"unicode"

	"go.opentelemetry.io/collector/consumer/pdata"
)

// constants for tags
const (
	// maximum for tag string lengths
	MaxTagLength = 200
	// DefaultServiceName is the default name we assign a service if it's missing and we have no reasonable fallback
	// From: https://github.com/DataDog/datadog-agent/blob/eab0dde41fe3a069a65c33d82a81b1ef1cf6b3bc/pkg/trace/traceutil/normalize.go#L15
	DefaultServiceName string = "unnamed-otel-service"
)

// NormalizeSpanName returns a cleaned up, normalized span name. Span names are used to formulate tags,
// and they also are used throughout the UI to connect metrics and traces. This helper function will:
//
// 	1. Convert to all lowercase unicode string
// 	2. Convert bad characters to underscores
// 	3. Dedupe contiguous underscores
// 	4. Remove leading non-alpha chars
// 	5. Truncate to MaxTagLength (200) characters
// 	6. Strip trailing underscores
//

func NormalizeSpanName(tag string, isService bool) string {
	// unless you just throw out unicode, this is already as fast as it gets
	bufSize := len(tag)
	if bufSize > MaxTagLength {
		bufSize = MaxTagLength // Limit size of allocation
	}
	var buf strings.Builder
	buf.Grow(bufSize)

	lastWasUnderscore := false

	for i, c := range tag {
		// Bail early if the tag contains a lot of non-letter/digit characters.
		// Let us assume if a tag is test🍣🍣[.,...], it's unlikely to be properly formated tag.
		// Max tag length matches backend constraint.
		if i > 2*MaxTagLength {
			break
		}
		// fast path for len check
		if buf.Len() >= MaxTagLength {
			break
		}
		// fast path for ascii alphabetic chars
		switch {
		case c >= 'a' && c <= 'z':
			buf.WriteRune(c)
			lastWasUnderscore = false
			continue
		case c >= 'A' && c <= 'Z':
			c -= 'A' - 'a'
			buf.WriteRune(c)
			lastWasUnderscore = false
			continue
		}

		c = unicode.ToLower(c)
		switch {
		// handle always valid cases
		case unicode.IsLetter(c):
			buf.WriteRune(c)
			lastWasUnderscore = false
		// skip any characters that can't start the string
		case buf.Len() == 0:
			continue
		// handle valid characters that can't start the string.
		// '-' creates issues in the UI so we skip it
		case unicode.IsDigit(c) || c == '.':
			buf.WriteRune(c)
			lastWasUnderscore = false
		// '-' only creates issues for span operation names not service names
		case c == '-' && isService:
			buf.WriteRune(c)
			lastWasUnderscore = false
		// convert anything else to underscores (including underscores), but only allow one in a row.
		case !lastWasUnderscore:
			buf.WriteRune('_')
			lastWasUnderscore = true
		}
	}

	s := buf.String()

	// strip trailing underscores
	if lastWasUnderscore {
		return s[:len(s)-1]
	}

	return s
}

// NormalizeSpanKind returns a span kind with the SPAN_KIND prefix trimmed off
func NormalizeSpanKind(kind pdata.SpanKind) string {
	return strings.TrimPrefix(kind.String(), "SPAN_KIND_")
}

// NormalizeServiceName returns a span service name normalized to remove invalid characters
// TODO: we'd like to move to the datadog-agent traceutil version of this once it's available in the exportable package
// https://github.com/DataDog/datadog-agent/blob/eab0dde41fe3a069a65c33d82a81b1ef1cf6b3bc/pkg/trace/traceutil/normalize.go#L52
func NormalizeServiceName(service string) string {
	if service == "" {
		return DefaultServiceName
	}

	s := NormalizeSpanName(service, true)

	if s == "" {
		return DefaultServiceName
	}

	return s
}
