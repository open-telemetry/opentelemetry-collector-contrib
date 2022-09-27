// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/logs"

import (
	"encoding/binary"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-agent/pkg/otlp/model/attributes"
	"github.com/DataDog/datadog-agent/pkg/otlp/model/source"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

const (
	// This set of constants specify the keys of the attributes that will be used to preserve
	// the original OpenTelemetry logs attributes.
	otelNamespace      = "otel"
	otelTraceID        = otelNamespace + ".trace_id"
	otelSpanID         = otelNamespace + ".span_id"
	otelSeverityNumber = otelNamespace + ".severity_number"
	otelSeverityText   = otelNamespace + ".severity_text"
	otelTimestamp      = otelNamespace + ".timestamp"
)
const (
	// This set of constants specify the keys of the attributes that will be used to represent Datadog
	// counterparts to the OpenTelemetry Logs attributes.
	ddNamespace = "dd"
	ddTraceID   = ddNamespace + ".trace_id"
	ddSpanID    = ddNamespace + ".span_id"
	ddStatus    = "status"
	ddTimestamp = "@timestamp"
)

const (
	logLevelTrace = "trace"
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
	logLevelFatal = "fatal"
)

// Transform converts the log record in lr, which came in with the resource in res to a Datadog log item.
// the variable specifies if the log body should be sent as an attribute or as a plain message.
func Transform(lr plog.LogRecord, res pcommon.Resource) datadogV2.HTTPLogItem {
	host, service := extractHostNameAndServiceName(res.Attributes(), lr.Attributes())

	l := datadogV2.HTTPLogItem{
		AdditionalProperties: make(map[string]string),
	}
	if host != "" {
		l.Hostname = datadog.PtrString(host)
	}
	if service != "" {
		l.Service = datadog.PtrString(service)
	}

	// we need to set log attributes as AdditionalProperties
	// AdditionalProperties are treated as Datadog Log Attributes
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		switch strings.ToLower(k) {
		case "msg", "message":
			l.Message = v.AsString()
		default:
			l.AdditionalProperties[k] = v.AsString()
		}
		return true
	})
	if !lr.TraceID().IsEmpty() {
		l.AdditionalProperties[ddTraceID] = strconv.FormatUint(traceIDToUint64(lr.TraceID()), 10)
		l.AdditionalProperties[otelTraceID] = lr.TraceID().HexString()
	}
	if !lr.SpanID().IsEmpty() {
		l.AdditionalProperties[ddSpanID] = strconv.FormatUint(spanIDToUint64(lr.SpanID()), 10)
		l.AdditionalProperties[otelSpanID] = lr.SpanID().HexString()
	}
	var status string

	// we want to use the serverity that client has set on the log and let datadog backend
	// decide the appropriate level
	if lr.SeverityText() != "" {
		status = lr.SeverityText()
		l.AdditionalProperties[otelSeverityText] = lr.SeverityText()
	} else if lr.SeverityNumber() != 0 {
		status = statusFromSeverityNumber(lr.SeverityNumber())
	}
	l.AdditionalProperties[ddStatus] = status
	if lr.SeverityNumber() != 0 {
		l.AdditionalProperties[otelSeverityNumber] = strconv.Itoa(int(lr.SeverityNumber()))
	}
	// for Datadog to use the same timestamp we need to set the additional property of "@timestamp"
	if lr.Timestamp() != 0 {
		// we are retaining the nano second precision in this property
		l.AdditionalProperties[otelTimestamp] = strconv.FormatInt(lr.Timestamp().AsTime().UnixNano(), 10)
		l.AdditionalProperties[ddTimestamp] = lr.Timestamp().AsTime().Format(time.RFC3339)
	}
	if l.Message == "" {
		// set the Message to the Body in case it wasn't already parsed as part of the attributes
		l.Message = lr.Body().AsString()
	}

	var tags = attributes.TagsFromAttributes(res.Attributes())
	if len(tags) > 0 {
		tagStr := strings.Join(tags, ",")
		l.Ddtags = datadog.PtrString(tagStr)
	}
	return l
}

func extractHostNameAndServiceName(resourceAttrs pcommon.Map, logAttrs pcommon.Map) (host string, service string) {
	if src, ok := attributes.SourceFromAttributes(resourceAttrs, true); ok && src.Kind == source.HostnameKind {
		host = src.Identifier
	}
	// hostName is blank from resource
	// we need to derive from log attributes
	if host == "" {
		if src, ok := attributes.SourceFromAttributes(logAttrs, true); ok && src.Kind == source.HostnameKind {
			host = src.Identifier
		}
	}
	if s, ok := resourceAttrs.Get(conventions.AttributeServiceName); ok {
		service = s.AsString()
	}
	// serviceName is blank from resource
	// we need to derive from log attributes
	if service == "" {
		if s, ok := logAttrs.Get(conventions.AttributeServiceName); ok {
			service = s.AsString()
		}
	}
	return host, service
}

// traceIDToUint64 converts 128bit traceId to 64 bit uint64
func traceIDToUint64(b [16]byte) uint64 {
	return binary.BigEndian.Uint64(b[len(b)-8:])
}

// spanIDToUint64 converts byte array to uint64
func spanIDToUint64(b [8]byte) uint64 {
	return binary.BigEndian.Uint64(b[:])
}

// statusFromSeverityNumber converts the severity number to log level
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#field-severitynumber
// this is not exactly datadog log levels , but derived from range name from above link
// see https://docs.datadoghq.com/logs/log_configuration/processors/?tab=ui#log-status-remapper for details on how it maps to datadog level
func statusFromSeverityNumber(severity plog.SeverityNumber) string {
	switch {
	case severity <= 4:
		return logLevelTrace
	case severity <= 8:
		return logLevelDebug
	case severity <= 12:
		return logLevelInfo
	case severity <= 16:
		return logLevelWarn
	case severity <= 20:
		return logLevelError
	case severity <= 24:
		return logLevelFatal
	default:
		// By default, treat this as error
		return logLevelError
	}
}
