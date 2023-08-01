// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter"

import (
	"encoding/json"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"google.golang.org/protobuf/proto"

	cls "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter/proto"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	traceIDField = "traceID"
	spanIDField  = "spanID"

	clsLogTimeUnixNano   = "timeUnixNano"
	clsLogSeverityNumber = "severityNumber"
	clsLogSeverityText   = "severityText"
	clsLogContent        = "content"
	clsLogAttribute      = "attribute"
	clsLogFlags          = "flags"
	clsLogResource       = "resource"
	clsLogHost           = "host"
	clsLogService        = "service"
	// shortcut for "otlp.instrumentation.library.name" "otlp.instrumentation.library.version"
	clsLogInstrumentationName    = "otlp.name"
	clsLogInstrumentationVersion = "otlp.version"
)

func convertLogs(ld plog.Logs) []*cls.Log {
	clsLogs := make([]*cls.Log, 0, ld.LogRecordCount())

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		ills := rl.ScopeLogs()
		resource := rl.Resource()
		resourceContents := resourceToLogContents(resource)
		for j := 0; j < ills.Len(); j++ {
			ils := ills.At(j)
			instrumentationLibraryContents := instrumentationLibraryToLogContents(ils.Scope())
			logs := ils.LogRecords()
			for j := 0; j < logs.Len(); j++ {
				clsLog := mapLogRecordToLogService(logs.At(j), resourceContents, instrumentationLibraryContents)
				if clsLog != nil {
					clsLogs = append(clsLogs, clsLog)
				}
			}
		}
	}

	return clsLogs
}

func resourceToLogContents(resource pcommon.Resource) []*cls.Log_Content {
	attrs := resource.Attributes()

	var hostname, serviceName string
	if host, ok := attrs.Get(conventions.AttributeHostName); ok {
		hostname = host.AsString()
	}

	if service, ok := attrs.Get(conventions.AttributeServiceName); ok {
		serviceName = service.AsString()
	}

	fields := map[string]interface{}{}
	attrs.Range(func(k string, v pcommon.Value) bool {
		if k == conventions.AttributeServiceName || k == conventions.AttributeHostName {
			return true
		}
		fields[k] = v.AsString()
		return true
	})
	attributeBuffer, err := json.Marshal(fields)
	if err != nil {
		return nil
	}

	return []*cls.Log_Content{
		{
			Key:   proto.String(clsLogHost),
			Value: proto.String(hostname),
		},
		{
			Key:   proto.String(clsLogService),
			Value: proto.String(serviceName),
		},
		{
			Key:   proto.String(clsLogResource),
			Value: proto.String(string(attributeBuffer)),
		},
	}
}

func instrumentationLibraryToLogContents(scope pcommon.InstrumentationScope) []*cls.Log_Content {
	return []*cls.Log_Content{
		{
			Key:   proto.String(clsLogInstrumentationName),
			Value: proto.String(scope.Name()),
		},
		{
			Key:   proto.String(clsLogInstrumentationVersion),
			Value: proto.String(scope.Version()),
		},
	}
}

func mapLogRecordToLogService(lr plog.LogRecord,
	resourceContents,
	instrumentationLibraryContents []*cls.Log_Content) *cls.Log {
	if lr.Body().Type() == pcommon.ValueTypeEmpty {
		return nil
	}
	var clsLog cls.Log

	// pre alloc, refine if logContent's len > 16
	preAllocCount := 16
	clsLog.Contents = make([]*cls.Log_Content, 0, preAllocCount+len(resourceContents)+len(instrumentationLibraryContents))

	fields := map[string]interface{}{}
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		fields[k] = v.AsString()
		return true
	})
	attributeBuffer, err := json.Marshal(fields)
	if err != nil {
		return nil
	}

	contentsBuffer := []*cls.Log_Content{
		{
			Key:   proto.String(clsLogTimeUnixNano),
			Value: proto.String(strconv.FormatUint(uint64(lr.Timestamp()), 10)),
		},
		{
			Key:   proto.String(clsLogSeverityNumber),
			Value: proto.String(strconv.FormatInt(int64(lr.SeverityNumber()), 10)),
		},
		{
			Key:   proto.String(clsLogSeverityText),
			Value: proto.String(lr.SeverityText()),
		},
		{
			Key:   proto.String(clsLogAttribute),
			Value: proto.String(string(attributeBuffer)),
		},
		{
			Key:   proto.String(clsLogContent),
			Value: proto.String(lr.Body().AsString()),
		},
		{
			Key:   proto.String(clsLogFlags),
			Value: proto.String(strconv.FormatUint(uint64(lr.Flags()), 16)),
		},
		{
			Key:   proto.String(traceIDField),
			Value: proto.String(traceutil.TraceIDToHexOrEmptyString(lr.TraceID())),
		},
		{
			Key:   proto.String(spanIDField),
			Value: proto.String(traceutil.SpanIDToHexOrEmptyString(lr.SpanID())),
		},
	}

	clsLog.Contents = append(clsLog.Contents, resourceContents...)
	clsLog.Contents = append(clsLog.Contents, instrumentationLibraryContents...)
	clsLog.Contents = append(clsLog.Contents, contentsBuffer...)

	if lr.Timestamp() > 0 {
		// convert time nano to time seconds
		clsLog.Time = proto.Int64(int64(lr.Timestamp() / 1000000000))
	} else {
		clsLog.Time = proto.Int64(time.Now().Unix())
	}

	return &clsLog
}
