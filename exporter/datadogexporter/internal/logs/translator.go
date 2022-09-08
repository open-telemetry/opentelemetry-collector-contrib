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
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/DataDog/datadog-agent/pkg/otlp/model/attributes"
	"github.com/DataDog/datadog-agent/pkg/otlp/model/source"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

const (
	namespace          = "otel"
	otelTraceID        = namespace + ".trace_id"
	otelSpandID        = namespace + ".span_id"
	otelSeverityNumber = namespace + ".serverity_number"
	otelSeverityText   = namespace + ".serverity_text"
	otelTimestamp      = namespace + ".timestamp"

	ddStatus    = "status"
	ddTraceID   = "dd.trace_id"
	ddSpanID    = "dd.span_id"
	ddTimestamp = "@timestamp"
)

const (
	LogLevelTrace = "trace"
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)

// Transform is responsible to convert LogRecord to datadog format
func Transform(lr plog.LogRecord, res pcommon.Resource) datadogV2.HTTPLogItem {
	hostName, serviceName := extractHostNameService(res)

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
		l.AdditionalProperties[ddTraceID] = convertTraceID(lr.TraceID().HexString())
		l.AdditionalProperties[otelTraceID] = lr.TraceID().HexString()
	}
	if !lr.SpanID().IsEmpty() {
		l.AdditionalProperties[ddSpanID] = convertTraceID(lr.SpanID().HexString())
		l.AdditionalProperties[otelSpandID] = lr.SpanID().HexString()
	}
	var status string

	if lr.SeverityText() != "" {
		status = lr.SeverityText()
		l.AdditionalProperties[otelSeverityText] = lr.SeverityText()
	} else {
		status = deriveStatus(int(lr.SeverityNumber()))
	}
	l.AdditionalProperties[ddStatus] = status

	if lr.SeverityNumber() != 0 {
		l.AdditionalProperties[otelSeverityNumber] = fmt.Sprintf("%d", lr.SeverityNumber())
	}

	// for datadog to use the same timestamp we need to set the additional property of "@timestamp"
	if lr.Timestamp() != 0 {
		// we are retaining the nano second precision in this property
		l.AdditionalProperties[otelTimestamp] = fmt.Sprintf("%d", lr.Timestamp())
		l.AdditionalProperties[ddTimestamp] = lr.Timestamp().AsTime().Format(time.RFC3339)
	}

	// TODO:  in case json parser is enabled , we shouldn't set Message
	// When we enable json parser, all the log attributes would be added as AdditionalProperties
	l.Message = lr.Body().AsString()

	var tags = attributes.TagsFromAttributes(res.Attributes())
	if len(tags) > 0 {
		tagStr := strings.Join(tags, ",")
		l.Ddtags = datadog.PtrString(tagStr)
	}
	return l
}

func extractHostNameService(r pcommon.Resource) (hostName string, serviceName string) {
	attr := r.Attributes()
	if src, ok := attributes.SourceFromAttributes(attr, true); ok && src.Kind == source.HostnameKind {
		hostName = src.Identifier
	}

	if s, ok := attr.Get(conventions.AttributeServiceName); ok {
		serviceName = s.AsString()
	}

	return hostName, serviceName
}

// convertTraceID would convert 128 bit otel TraceID to 64 bit datadog TraceID
func convertTraceID(id string) string {
	if len(id) < 16 {
		return ""
	}
	if len(id) > 16 {
		id = id[16:]
	}
	intValue, err := strconv.ParseUint(id, 16, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(intValue, 10)
}

// deriveStatus converts the severity number to log level
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#field-severitynumber
func deriveStatus(severity int) string {
	if severity < 5 {
		return LogLevelTrace
	}
	if severity < 8 {
		return LogLevelDebug
	}
	if severity < 12 {
		return LogLevelInfo
	}
	if severity < 16 {
		return LogLevelWarn
	}
	if severity < 20 {
		return LogLevelError
	}
	if severity < 24 {
		return LogLevelFatal
	}
	// By default, treat this as error
	return LogLevelError
}
