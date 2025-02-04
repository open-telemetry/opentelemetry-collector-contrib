// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

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

// copyOtelAttrs copies values from HecToOtelAttrs to OtelAttrsToHec struct.
func copyOtelAttrs(config *Config) {
	defaultCfg := createDefaultConfig().(*Config)
	if config.OtelAttrsToHec.Equal(defaultCfg.OtelAttrsToHec) {
		if !config.HecToOtelAttrs.Equal(defaultCfg.HecToOtelAttrs) {
			// Copy settings to ease deprecation of HecToOtelAttrs.
			config.OtelAttrsToHec = config.HecToOtelAttrs
		}
	} else {
		if !config.HecToOtelAttrs.Equal(defaultCfg.HecToOtelAttrs) {
			// Replace all default fields in OtelAttrsToHec.
			if config.OtelAttrsToHec.Source == defaultCfg.OtelAttrsToHec.Source {
				config.OtelAttrsToHec.Source = config.HecToOtelAttrs.Source
			}
			if config.OtelAttrsToHec.SourceType == defaultCfg.OtelAttrsToHec.SourceType {
				config.OtelAttrsToHec.SourceType = config.HecToOtelAttrs.SourceType
			}
			if config.OtelAttrsToHec.Index == defaultCfg.OtelAttrsToHec.Index {
				config.OtelAttrsToHec.Index = config.HecToOtelAttrs.Index
			}
			if config.OtelAttrsToHec.Host == defaultCfg.OtelAttrsToHec.Host {
				config.OtelAttrsToHec.Host = config.HecToOtelAttrs.Host
			}
		}
	}
}

func mapLogRecordToSplunkEvent(res pcommon.Resource, lr plog.LogRecord, config *Config) *splunk.Event {
	body := lr.Body().AsRaw()
	if body == nil || body == "" {
		// events with no body are rejected by Splunk.
		return nil
	}

	// Manage the deprecation of HecToOtelAttrs config parameters.
	// TODO: remove this once HecToOtelAttrs is removed from Config.
	copyOtelAttrs(config)

	host := unknownHostName
	source := config.Source
	sourcetype := config.SourceType
	index := config.Index
	fields := map[string]any{}
	sourceKey := config.OtelAttrsToHec.Source
	sourceTypeKey := config.OtelAttrsToHec.SourceType
	indexKey := config.OtelAttrsToHec.Index
	hostKey := config.OtelAttrsToHec.Host
	severityTextKey := config.HecFields.SeverityText
	severityNumberKey := config.HecFields.SeverityNumber
	if spanID := lr.SpanID(); !spanID.IsEmpty() {
		fields[spanIDFieldKey] = hex.EncodeToString(spanID[:])
	}
	if traceID := lr.TraceID(); !traceID.IsEmpty() {
		fields[traceIDFieldKey] = hex.EncodeToString(traceID[:])
	}
	if lr.SeverityText() != "" {
		fields[severityTextKey] = lr.SeverityText()
	}
	if lr.SeverityNumber() != plog.SeverityNumberUnspecified {
		fields[severityNumberKey] = lr.SeverityNumber()
	}

	res.Attributes().Range(func(k string, v pcommon.Value) bool {
		switch k {
		case hostKey:
			host = v.Str()
		case sourceKey:
			source = v.Str()
		case sourceTypeKey:
			sourcetype = v.Str()
		case indexKey:
			index = v.Str()
		case splunk.HecTokenLabel:
			// ignore
		default:
			mergeValue(fields, k, v.AsRaw())
		}
		return true
	})
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		switch k {
		case hostKey:
			host = v.Str()
		case sourceKey:
			source = v.Str()
		case sourceTypeKey:
			sourcetype = v.Str()
		case indexKey:
			index = v.Str()
		case splunk.HecTokenLabel:
			// ignore
		default:
			mergeValue(fields, k, v.AsRaw())
		}
		return true
	})

	return &splunk.Event{
		Time:       nanoTimestampToEpochMilliseconds(lr.Timestamp()),
		Host:       host,
		Source:     source,
		SourceType: sourcetype,
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
