// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"

import (
	"encoding/json"
	"strconv"
	"time"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	slsLogTimeUnixNano   = "timeUnixNano"
	slsLogSeverityNumber = "severityNumber"
	slsLogSeverityText   = "severityText"
	slsLogContent        = "content"
	slsLogAttribute      = "attribute"
	slsLogFlags          = "flags"
	slsLogResource       = "resource"
	slsLogHost           = "host"
	slsLogService        = "service"
	// shortcut for "otlp.instrumentation.library.name" "otlp.instrumentation.library.version"
	slsLogInstrumentationName    = "otlp.name"
	slsLogInstrumentationVersion = "otlp.version"
)

func logDataToLogService(ld plog.Logs) []*sls.Log {
	var slsLogs []*sls.Log
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		sl := rl.ScopeLogs()
		resource := rl.Resource()
		resourceContents := resourceToLogContents(resource)
		for j := 0; j < sl.Len(); j++ {
			ils := sl.At(j)
			instrumentationLibraryContents := instrumentationScopeToLogContents(ils.Scope())
			logs := ils.LogRecords()
			for j := 0; j < logs.Len(); j++ {
				slsLog := mapLogRecordToLogService(logs.At(j), resourceContents, instrumentationLibraryContents)
				if slsLog != nil {
					slsLogs = append(slsLogs, slsLog)
				}
			}
		}
	}

	return slsLogs
}

func resourceToLogContents(resource pcommon.Resource) []*sls.LogContent {
	logContents := make([]*sls.LogContent, 3)
	attrs := resource.Attributes()
	if hostName, ok := attrs.Get(string(conventions.HostNameKey)); ok {
		logContents[0] = &sls.LogContent{
			Key:   new(slsLogHost),
			Value: new(hostName.AsString()),
		}
	} else {
		logContents[0] = &sls.LogContent{
			Key:   new(slsLogHost),
			Value: new(""),
		}
	}

	if serviceName, ok := attrs.Get(string(conventions.ServiceNameKey)); ok {
		logContents[1] = &sls.LogContent{
			Key:   new(slsLogService),
			Value: new(serviceName.AsString()),
		}
	} else {
		logContents[1] = &sls.LogContent{
			Key:   new(slsLogService),
			Value: new(""),
		}
	}

	fields := map[string]any{}
	for k, v := range attrs.All() {
		if k == string(conventions.ServiceNameKey) || k == string(conventions.HostNameKey) {
			continue
		}
		fields[k] = v.AsString()
	}
	attributeBuffer, _ := json.Marshal(fields)
	logContents[2] = &sls.LogContent{
		Key:   new(slsLogResource),
		Value: new(string(attributeBuffer)),
	}

	return logContents
}

func instrumentationScopeToLogContents(instrumentationScope pcommon.InstrumentationScope) []*sls.LogContent {
	logContents := make([]*sls.LogContent, 2)
	logContents[0] = &sls.LogContent{
		Key:   new(slsLogInstrumentationName),
		Value: new(instrumentationScope.Name()),
	}
	logContents[1] = &sls.LogContent{
		Key:   new(slsLogInstrumentationVersion),
		Value: new(instrumentationScope.Version()),
	}
	return logContents
}

func mapLogRecordToLogService(lr plog.LogRecord,
	resourceContents,
	instrumentationLibraryContents []*sls.LogContent,
) *sls.Log {
	if lr.Body().Type() == pcommon.ValueTypeEmpty {
		return nil
	}
	var slsLog sls.Log

	// pre alloc, refine if logContent's len > 16
	preAllocCount := 16
	slsLog.Contents = make([]*sls.LogContent, 0, preAllocCount+len(resourceContents)+len(instrumentationLibraryContents))
	contentsBuffer := make([]sls.LogContent, 0, preAllocCount)

	slsLog.Contents = append(slsLog.Contents, resourceContents...)
	slsLog.Contents = append(slsLog.Contents, instrumentationLibraryContents...)

	contentsBuffer = append(contentsBuffer,
		sls.LogContent{
			Key:   new(slsLogTimeUnixNano),
			Value: new(strconv.FormatUint(uint64(lr.Timestamp()), 10)),
		},
		sls.LogContent{
			Key:   new(slsLogSeverityNumber),
			Value: new(strconv.FormatInt(int64(lr.SeverityNumber()), 10)),
		},
		sls.LogContent{
			Key:   new(slsLogSeverityText),
			Value: new(lr.SeverityText()),
		})

	fields := map[string]any{}
	for k, v := range lr.Attributes().All() {
		fields[k] = v.AsString()
	}
	attributeBuffer, _ := json.Marshal(fields)
	contentsBuffer = append(contentsBuffer,
		sls.LogContent{
			Key:   new(slsLogAttribute),
			Value: new(string(attributeBuffer)),
		},
		sls.LogContent{
			Key:   new(slsLogContent),
			Value: new(lr.Body().AsString()),
		},
		sls.LogContent{
			Key:   new(slsLogFlags),
			Value: new(strconv.FormatUint(uint64(lr.Flags()), 16)),
		},
		sls.LogContent{
			Key:   new(traceIDField),
			Value: new(traceutil.TraceIDToHexOrEmptyString(lr.TraceID())),
		},
		sls.LogContent{
			Key:   new(spanIDField),
			Value: new(traceutil.SpanIDToHexOrEmptyString(lr.SpanID())),
		})

	for i := range contentsBuffer {
		slsLog.Contents = append(slsLog.Contents, &contentsBuffer[i])
	}

	if lr.Timestamp() > 0 {
		// convert time nano to time seconds
		slsLog.Time = new(uint32(lr.Timestamp() / 1000000000))
	} else {
		slsLog.Time = new(uint32(time.Now().Unix()))
	}

	return &slsLog
}
