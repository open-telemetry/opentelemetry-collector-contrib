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
	namespace          = "otel"
	otelTraceID        = namespace + ".trace_id"
	otelSpanID         = namespace + ".span_id"
	otelSeverityNumber = namespace + ".severity_number"
	otelSeverityText   = namespace + ".severity_text"
	otelTimestamp      = namespace + ".timestamp"

	ddStatus    = "status"
	ddTraceID   = "dd.trace_id"
	ddSpanID    = "dd.span_id"
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

// Transform is responsible to convert LogRecord to datadog format
func Transform(lr plog.LogRecord, res pcommon.Resource, sendLogRecordBody bool) datadogV2.HTTPLogItem {
	hostName, serviceName := extractHostNameAndServiceName(res.Attributes(), lr.Attributes())

	l := datadogV2.HTTPLogItem{
		AdditionalProperties: make(map[string]string),
	}
	if hostName != "" {
		l.Hostname = datadog.PtrString(hostName)
	}

	if serviceName != "" {
		l.Service = datadog.PtrString(serviceName)
	}

	// we need to set log attributes as AdditionalProperties
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		l.AdditionalProperties[k] = v.AsString()
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
		status = derviveStatusFromSeverityNumber(lr.SeverityNumber())
	}

	l.AdditionalProperties[ddStatus] = status
	// if SeverityNumber is set , we want to retain it
	if lr.SeverityNumber() != 0 {
		l.AdditionalProperties[otelSeverityNumber] = strconv.Itoa(int(lr.SeverityNumber()))
	}

	// for datadog to use the same timestamp we need to set the additional property of "@timestamp"
	if lr.Timestamp() != 0 {
		// we are retaining the nano second precision in this property
		l.AdditionalProperties[otelTimestamp] = strconv.FormatInt(lr.Timestamp().AsTime().UnixNano(), 10)
		l.AdditionalProperties[ddTimestamp] = lr.Timestamp().AsTime().Format(time.RFC3339)
	}

	if sendLogRecordBody {
		// set the Message
		l.Message = lr.Body().AsString()
	}

	var tags = attributes.TagsFromAttributes(res.Attributes())
	if len(tags) > 0 {
		tagStr := strings.Join(tags, ",")
		l.Ddtags = datadog.PtrString(tagStr)
	}
	return l
}

func extractHostNameAndServiceName(resourceAttrs pcommon.Map, logAttrs pcommon.Map) (hostName string, serviceName string) {
	if src, ok := attributes.SourceFromAttributes(resourceAttrs, true); ok && src.Kind == source.HostnameKind {
		hostName = src.Identifier
	}

	// hostName is blank from resource
	// we need to derive from log attributes
	if hostName == "" {
		if src, ok := attributes.SourceFromAttributes(logAttrs, true); ok && src.Kind == source.HostnameKind {
			hostName = src.Identifier
		}
	}

	if s, ok := resourceAttrs.Get(conventions.AttributeServiceName); ok {
		serviceName = s.AsString()
	}

	// serviceName is blank from resource
	// we need to derive from log attributes
	if serviceName == "" {
		if s, ok := logAttrs.Get(conventions.AttributeServiceName); ok {
			serviceName = s.AsString()
		}
	}

	return hostName, serviceName
}

// traceIDToUint64 converts 128bit traceId to 64 bit uint64
func traceIDToUint64(b [16]byte) uint64 {
	return binary.BigEndian.Uint64(b[len(b)-8:])
}

// spanIDToUint64 converts byte array to uint64
func spanIDToUint64(b [8]byte) uint64 {
	return binary.BigEndian.Uint64(b[:])
}

// derviveStatusFromSeverityNumber converts the severity number to log level
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#field-severitynumber
// this is not exactly datadog log levels , but derived from range name from above link
// see https://docs.datadoghq.com/logs/log_configuration/processors/?tab=ui#log-status-remapper for details on how it maps to datadog level
func derviveStatusFromSeverityNumber(severity plog.SeverityNumber) string {
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
