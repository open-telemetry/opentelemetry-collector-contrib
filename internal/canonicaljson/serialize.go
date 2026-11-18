// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package canonicaljson // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/canonicaljson"

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gowebpki/jcs"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Default values
const (
	DefaultMaxDepth      = 128
	DefaultMaxInputBytes = 1 << 21 // 2 MiB
)

// SerializeOptions controls log record and span serialization behavior.
type SerializeOptions struct {
	// ExcludePrefixes lists attribute prefixes to exclude from serialization
	// e.g., ["audit.integrity.", "audit.verification."]
	ExcludePrefixes []string

	// MaxDepth caps recursion depth for nested Slice/Map values
	// Default: 128
	MaxDepth int

	// MaxInputBytes caps the serialized JSON size before JCS canonicalization
	// Default: 2 MiB (1 << 21)
	MaxInputBytes int
}

// SerializeLogRecordOptions controls log record serialization behavior.
type SerializeLogRecordOptions = SerializeOptions

// SerializeSpanOptions controls span serialization behavior.
type SerializeSpanOptions = SerializeOptions

func (opts SerializeOptions) withDefaults() SerializeOptions {
	if opts.MaxDepth == 0 {
		opts.MaxDepth = DefaultMaxDepth
	}
	if opts.MaxInputBytes == 0 {
		opts.MaxInputBytes = DefaultMaxInputBytes
	}
	return opts
}

// SerializeLogRecord produces RFC 8785 (JCS) canonical JSON of a log record.
//
// Fields included in serialization (per OTLP LogRecord):
//   - event_name (EventName)
//   - body (Body)
//   - timestamp (Timestamp as Unix nanoseconds string)
//   - observed_timestamp (ObservedTimestamp as Unix nanoseconds string)
//   - trace_id (TraceID as hex string)
//   - span_id (SpanID as hex string)
//   - attributes (Attributes, filtered by ExcludePrefixes)
//
// Fields intentionally excluded from serialization:
//   - Attributes matching any prefix in opts.ExcludePrefixes
//   - SeverityNumber / SeverityText (audit metadata, not signed content)
//   - Flags (W3C trace-context sampling bit, transport-level metadata)
//   - DroppedAttributesCount (pipeline metadata, not original content)
//
// Note: This field list is intentionally hard-coded rather than using reflection
// to ensure cryptographic stability. Any new OTLP LogRecord fields require explicit
// review to determine if they should be part of the signed payload.
//
// Returns canonical JSON bytes or an error if:
//   - Nesting depth exceeds MaxDepth
//   - Serialized size exceeds MaxInputBytes
//   - Invalid UTF-8 is encountered
//   - JCS transformation fails
func SerializeLogRecord(lr plog.LogRecord, opts SerializeLogRecordOptions) ([]byte, error) {
	opts = opts.withDefaults()

	data := make(map[string]any)

	if lr.EventName() != "" {
		if err := checkUTF8(lr.EventName(), "event name"); err != nil {
			return nil, err
		}
		data["event_name"] = lr.EventName()
	}

	if lr.Body().Type() != pcommon.ValueTypeEmpty {
		body, err := valueToInterface(lr.Body(), 0, opts.MaxDepth)
		if err != nil {
			return nil, fmt.Errorf("log record body: %w", err)
		}
		data["body"] = body
	}

	if lr.Timestamp() != 0 {
		data["timestamp"] = Nanos(lr.Timestamp())
	}

	if lr.ObservedTimestamp() != 0 {
		data["observed_timestamp"] = Nanos(lr.ObservedTimestamp())
	}

	if !lr.TraceID().IsEmpty() {
		data["trace_id"] = lr.TraceID().String()
	}

	if !lr.SpanID().IsEmpty() {
		data["span_id"] = lr.SpanID().String()
	}

	if err := putAttributes(data, lr.Attributes(), opts); err != nil {
		return nil, err
	}

	return marshalJCS(data, opts.MaxInputBytes)
}

// SerializeSpan produces RFC 8785 (JCS) canonical JSON of a span.
//
// Fields included in serialization:
//   - trace_id, span_id, parent_span_id (hex strings, if non-empty)
//   - name (if non-empty) and kind (always included as a number)
//   - start_time and end_time (unsigned Unix nanoseconds strings, if non-zero)
//   - attributes (filtered by ExcludePrefixes, if non-empty)
//   - events (in original order, with name, timestamp, and filtered attributes)
//   - links (in original order, with trace_id, span_id, and filtered attributes)
//   - status (always included, with numeric code and non-empty message)
//
// TraceState and Flags on spans and links, all dropped counts, and attributes
// matching ExcludePrefixes on spans, events, and links are intentionally excluded.
// Empty event and link fields are omitted, but their entries are retained.
// This schema is hard-coded for cryptographic stability; new OTLP fields require
// explicit review before inclusion.
//
// Returns an error for excessive nesting depth or serialized size, invalid
// UTF-8, or a failed JCS transformation, as with SerializeLogRecord.
func SerializeSpan(span ptrace.Span, opts SerializeSpanOptions) ([]byte, error) {
	opts = opts.withDefaults()
	data := make(map[string]any)

	if !span.TraceID().IsEmpty() {
		data["trace_id"] = span.TraceID().String()
	}
	if !span.SpanID().IsEmpty() {
		data["span_id"] = span.SpanID().String()
	}
	if !span.ParentSpanID().IsEmpty() {
		data["parent_span_id"] = span.ParentSpanID().String()
	}
	if span.Name() != "" {
		if err := checkUTF8(span.Name(), "span name"); err != nil {
			return nil, err
		}
		data["name"] = span.Name()
	}
	data["kind"] = int32(span.Kind())
	if span.StartTimestamp() != 0 {
		data["start_time"] = Nanos(span.StartTimestamp())
	}
	if span.EndTimestamp() != 0 {
		data["end_time"] = Nanos(span.EndTimestamp())
	}

	if err := putAttributes(data, span.Attributes(), opts); err != nil {
		return nil, err
	}

	if span.Events().Len() > 0 {
		events := make([]any, 0, span.Events().Len())
		for i := 0; i < span.Events().Len(); i++ {
			ev := span.Events().At(i)
			event := make(map[string]any)
			if ev.Name() != "" {
				if err := checkUTF8(ev.Name(), "span event name"); err != nil {
					return nil, err
				}
				event["name"] = ev.Name()
			}
			if ev.Timestamp() != 0 {
				event["timestamp"] = Nanos(ev.Timestamp())
			}
			if err := putAttributes(event, ev.Attributes(), opts); err != nil {
				return nil, fmt.Errorf("span event %d: %w", i, err)
			}
			events = append(events, event)
		}
		data["events"] = events
	}

	if span.Links().Len() > 0 {
		links := make([]any, 0, span.Links().Len())
		for i := 0; i < span.Links().Len(); i++ {
			l := span.Links().At(i)
			link := make(map[string]any)
			if !l.TraceID().IsEmpty() {
				link["trace_id"] = l.TraceID().String()
			}
			if !l.SpanID().IsEmpty() {
				link["span_id"] = l.SpanID().String()
			}
			if err := putAttributes(link, l.Attributes(), opts); err != nil {
				return nil, fmt.Errorf("span link %d: %w", i, err)
			}
			links = append(links, link)
		}
		data["links"] = links
	}

	status := map[string]any{"code": int32(span.Status().Code())}
	if msg := span.Status().Message(); msg != "" {
		if err := checkUTF8(msg, "span status message"); err != nil {
			return nil, err
		}
		status["message"] = msg
	}
	data["status"] = status

	return marshalJCS(data, opts.MaxInputBytes)
}

// putAttributes adds non-empty attributes after filtering and validating them.
func putAttributes(data map[string]any, attributes pcommon.Map, opts SerializeOptions) error {
	attrs := make(map[string]any)
	var attrErr error
	attributes.Range(func(k string, v pcommon.Value) bool {
		// Check if key matches any exclude prefix
		for _, prefix := range opts.ExcludePrefixes {
			if strings.HasPrefix(k, prefix) {
				return true
			}
		}

		if err := checkUTF8(k, "attribute key"); err != nil {
			attrErr = err
			return false
		}
		val, err := valueToInterface(v, 0, opts.MaxDepth)
		if err != nil {
			attrErr = err
			return false
		}
		attrs[k] = val
		return true
	})
	if attrErr != nil {
		return attrErr
	}
	if len(attrs) > 0 {
		data["attributes"] = attrs
	}

	return nil
}

// marshalJCS produces RFC 8785 (JCS) canonical JSON bytes.
// json.Marshal sorts map keys (Go ≥ 1.12), then jcs.Transform normalizes
// number representation and validates the result per the JCS spec.
func marshalJCS(v any, maxInputBytes int) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxInputBytes {
		return nil, fmt.Errorf("serialized payload exceeds size limit (%d > %d bytes)", len(raw), maxInputBytes)
	}
	return jcs.Transform(raw)
}

// Nanos converts a pcommon.Timestamp to its string representation in nanoseconds.
func Nanos(ts pcommon.Timestamp) string {
	return strconv.FormatUint(uint64(ts), 10)
}
