// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/splunk"

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	// Keys are taken from https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/overview.md#trace-context-in-legacy-formats.
	// spanIDFieldKey is the key used in log event for the span id (if any).
	spanIDFieldKey = "span_id"
	// traceIDFieldKey is the key used in the log event for the trace id (if any).
	traceIDFieldKey = "trace_id"
)

func LogToSplunkEvent(res pcommon.Resource, lr plog.LogRecord, toOtelAttrs HecToOtelAttrs, toHecAttrs OtelToHecFields, source, sourceType, index string) *Event {
	body := lr.Body().AsRaw()
	if body == nil || body == "" {
		// events with no body are rejected by
		return nil
	}

	host := unknownHostName
	fields := map[string]any{}
	if spanID := lr.SpanID(); !spanID.IsEmpty() {
		fields[spanIDFieldKey] = hex.EncodeToString(spanID[:])
	}
	if traceID := lr.TraceID(); !traceID.IsEmpty() {
		fields[traceIDFieldKey] = hex.EncodeToString(traceID[:])
	}
	if lr.SeverityText() != "" {
		fields[toHecAttrs.SeverityText] = lr.SeverityText()
	}
	if lr.SeverityNumber() != plog.SeverityNumberUnspecified {
		fields[toHecAttrs.SeverityNumber] = lr.SeverityNumber()
	}

	for k, v := range res.Attributes().All() {
		switch k {
		case toOtelAttrs.Host:
			host = v.Str()
		case toOtelAttrs.Source:
			source = v.Str()
		case toOtelAttrs.SourceType:
			sourceType = v.Str()
		case toOtelAttrs.Index:
			index = v.Str()
		case splunk.HecTokenLabel:
			// ignore
		default:
			mergeValue(fields, k, v.AsRaw())
		}
	}
	for k, v := range lr.Attributes().All() {
		switch k {
		case toOtelAttrs.Host:
			host = v.Str()
		case toOtelAttrs.Source:
			source = v.Str()
		case toOtelAttrs.SourceType:
			sourceType = v.Str()
		case toOtelAttrs.Index:
			index = v.Str()
		case splunk.HecTokenLabel:
			// ignore
		default:
			mergeValue(fields, k, v.AsRaw())
		}
	}

	ts := lr.Timestamp()
	if ts == 0 {
		ts = lr.ObservedTimestamp()
	}

	return &Event{
		Time:       nanoTimestampToEpochMilliseconds(ts),
		Host:       host,
		Source:     source,
		SourceType: sourceType,
		Index:      index,
		Event:      body,
		Fields:     fields,
	}
}

// nanoTimestampToEpochMilliseconds transforms nanoseconds into <sec>.<ms>. For example, 1433188255.500 indicates 1433188255 seconds and 500 milliseconds after epoch.
func nanoTimestampToEpochMilliseconds(ts pcommon.Timestamp) float64 {
	return time.Duration(ts).Round(time.Millisecond).Seconds()
}

func mergeValue(dst map[string]any, k string, v any) {
	switch element := v.(type) {
	case []any:
		if isArrayFlat(element) {
			dst[k] = v
		} else {
			b, _ := json.Marshal(element)
			dst[k] = string(b)
		}
	case map[string]any:
		flattenAndMergeMap(element, dst, k)
	default:
		dst[k] = v
	}
}

func isArrayFlat(array []any) bool {
	for _, v := range array {
		switch v.(type) {
		case []any, map[string]any:
			return false
		}
	}
	return true
}

func flattenAndMergeMap(src, dst map[string]any, key string) {
	for k, v := range src {
		current := fmt.Sprintf("%s.%s", key, k)
		switch element := v.(type) {
		case map[string]any:
			flattenAndMergeMap(element, dst, current)
		case []any:
			if isArrayFlat(element) {
				dst[current] = element
			} else {
				b, _ := json.Marshal(element)
				dst[current] = string(b)
			}

		default:
			dst[current] = element
		}
	}
}
