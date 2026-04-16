// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/splunk"

import (
	"encoding/hex"
	"fmt"

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
	fields := make(map[string]any, lr.Attributes().Len()+res.Attributes().Len()+2)
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
			mergeValue(fields, k, v)
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
			mergeValue(fields, k, v)
		}
	}

	ts := lr.Timestamp()
	if ts == 0 {
		ts = lr.ObservedTimestamp()
	}

	return &Event{
		Time:       nanoToEpochSeconds(ts),
		Host:       host,
		Source:     source,
		SourceType: sourceType,
		Index:      index,
		Event:      body,
		Fields:     fields,
	}
}

func mergeValue(dst map[string]any, k string, v pcommon.Value) {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		dst[k] = nil
	case pcommon.ValueTypeStr:
		dst[k] = v.Str()
	case pcommon.ValueTypeBool:
		dst[k] = v.Bool()
	case pcommon.ValueTypeDouble:
		dst[k] = v.Double()
	case pcommon.ValueTypeInt:
		dst[k] = v.Int()
	case pcommon.ValueTypeBytes:
		dst[k] = v.Bytes().AsRaw()
	case pcommon.ValueTypeMap:
		flattenAndMergeMap(v.Map(), dst, k)
	case pcommon.ValueTypeSlice:
		if isArrayFlat(v.Slice()) {
			dst[k] = v.Slice().AsRaw()
		} else {
			b, _ := json.Marshal(v.Slice().AsRaw())
			dst[k] = string(b)
		}
	}
}

func isArrayFlat(array pcommon.Slice) bool {
	for _, v := range array.All() {
		switch v.Type() {
		case pcommon.ValueTypeSlice, pcommon.ValueTypeMap:
			return false
		}
	}
	return true
}

func flattenAndMergeMap(src pcommon.Map, dst map[string]any, key string) {
	for k, v := range src.All() {
		current := fmt.Sprintf("%s.%s", key, k)
		switch v.Type() {
		case pcommon.ValueTypeMap:
			flattenAndMergeMap(v.Map(), dst, current)
		case pcommon.ValueTypeSlice:
			if isArrayFlat(v.Slice()) {
				dst[current] = v.Slice().AsRaw()
			} else {
				b, _ := json.Marshal(v.Slice().AsRaw())
				dst[current] = string(b)
			}
		case pcommon.ValueTypeEmpty:
			dst[current] = nil
		case pcommon.ValueTypeStr:
			dst[current] = v.Str()
		case pcommon.ValueTypeBool:
			dst[current] = v.Bool()
		case pcommon.ValueTypeDouble:
			dst[current] = v.Double()
		case pcommon.ValueTypeInt:
			dst[current] = v.Int()
		case pcommon.ValueTypeBytes:
			dst[current] = v.Bytes().AsRaw()
		}
	}
}
