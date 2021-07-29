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
	"unicode/utf8"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"go.opentelemetry.io/collector/model/pdata"
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

// GetRoot extracts the root span from a trace
// From: https://github.com/DataDog/datadog-agent/blob/a6872e436681ea2136cf8a67465e99fdb4450519/pkg/trace/traceutil/trace.go#L27
func GetRoot(t *pb.APITrace) *pb.Span {
	// That should be caught beforehand
	spans := t.GetSpans()
	if len(spans) == 0 {
		return nil
	}
	// General case: go over all spans and check for one which matching parent
	parentIDToChild := map[uint64]*pb.Span{}

	for i := range spans {
		// Common case optimization: check for span with ParentID == 0, starting from the end,
		// since some clients report the root last
		j := len(spans) - 1 - i
		if spans[j].ParentID == 0 {
			return spans[j]
		}
		parentIDToChild[spans[j].ParentID] = spans[j]
	}

	for i := range spans {
		_, ok := parentIDToChild[spans[i].SpanID]

		if ok {
			delete(parentIDToChild, spans[i].SpanID)
		}
	}

	// TODO: pass logger into convertToDatadogTd, so that we can improve debug logging in translation.
	// For now it would require some rewrite, but ideally should log if we cant locate a root span

	// Have a safe behavior if there is multiple spans without a parent
	// Pick the first span without its parent
	for parentID := range parentIDToChild {
		return parentIDToChild[parentID]
	}

	// Gracefully fail with the last span of the trace
	return spans[len(spans)-1]
}

// TruncateUTF8 truncates the given string to make sure it uses less than limit bytes.
// If the last character is an utf8 character that would be splitten, it removes it
// entirely to make sure the resulting string is not broken.
// from: https://github.com/DataDog/datadog-agent/blob/140a4ee164261ef2245340c50371ba989fbeb038/pkg/trace/traceutil/truncate.go#L34-L49
func TruncateUTF8(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	var lastValidIndex int
	for i := range s {
		if i > limit {
			return s[:lastValidIndex]
		}
		lastValidIndex = i
	}
	return s
}

// NormalizeTag applies some normalization to ensure the tags match the backend requirements.
// Specifically used for env tag currently
// port from: https://github.com/DataDog/datadog-agent/blob/c87e93a75b1fc97f0691faf78ae8eb2c280d6f55/pkg/trace/traceutil/normalize.go#L89
func NormalizeTag(v string) string {
	// the algorithm works by creating a set of cuts marking start and end offsets in v
	// that have to be replaced with underscore (_)
	if len(v) == 0 {
		return ""
	}
	var (
		trim  int      // start character (if trimming)
		cuts  [][2]int // sections to discard: (start, end) pairs
		chars int      // number of characters processed
	)
	var (
		i    int  // current byte
		r    rune // current rune
		jump int  // tracks how many bytes the for range advances on its next iteration
	)
	tag := []byte(v)
	for i, r = range v {
		jump = utf8.RuneLen(r) // next i will be i+jump
		if r == utf8.RuneError {
			// On invalid UTF-8, the for range advances only 1 byte (see: https://golang.org/ref/spec#For_range (point 2)).
			// However, utf8.RuneError is equivalent to unicode.ReplacementChar so we should rely on utf8.DecodeRune to tell
			// us whether this is an actual error or just a unicode.ReplacementChar that was present in the string.
			_, width := utf8.DecodeRune(tag[i:])
			jump = width
		}
		// fast path; all letters (and colons) are ok
		switch {
		case r >= 'a' && r <= 'z' || r == ':':
			chars++
			goto end
		case r >= 'A' && r <= 'Z':
			// lower-case
			tag[i] += 'a' - 'A'
			chars++
			goto end
		}
		if unicode.IsUpper(r) {
			// lowercase this character
			if low := unicode.ToLower(r); utf8.RuneLen(r) == utf8.RuneLen(low) {
				// but only if the width of the lowercased character is the same;
				// there are some rare edge-cases where this is not the case, such
				// as \u017F (ſ)
				utf8.EncodeRune(tag[i:], low)
				r = low
			}
		}
		switch {
		case unicode.IsLetter(r):
			chars++
		case chars == 0:
			// this character can not start the string, trim
			trim = i + jump
			goto end
		case unicode.IsDigit(r) || r == '.' || r == '/' || r == '-':
			chars++
		default:
			// illegal character
			if n := len(cuts); n > 0 && cuts[n-1][1] >= i {
				// merge intersecting cuts
				cuts[n-1][1] += jump
			} else {
				// start a new cut
				cuts = append(cuts, [2]int{i, i + jump})
			}
		}
	end:
		if i+jump >= 2*MaxTagLength {
			// bail early if the tag contains a lot of non-letter/digit characters.
			// If a tag is test🍣🍣[...]🍣, then it's unlikely to be a properly formatted tag
			break
		}
		if chars >= MaxTagLength {
			// we've reached the maximum
			break
		}
	}

	tag = tag[trim : i+jump] // trim start and end
	if len(cuts) == 0 {
		// tag was ok, return it as it is
		return string(tag)
	}
	delta := trim // cut offsets delta
	for _, cut := range cuts {
		// start and end of cut, including delta from previous cuts:
		start, end := cut[0]-delta, cut[1]-delta

		if end >= len(tag) {
			// this cut includes the end of the string; discard it
			// completely and finish the loop.
			tag = tag[:start]
			break
		}
		// replace the beginning of the cut with '_'
		tag[start] = '_'
		if end-start == 1 {
			// nothing to discard
			continue
		}
		// discard remaining characters in the cut
		copy(tag[start+1:], tag[end:])

		// shorten the slice
		tag = tag[:len(tag)-(end-start)+1]

		// count the new delta for future cuts
		delta += cut[1] - cut[0] - 1
	}
	return string(tag)
}
