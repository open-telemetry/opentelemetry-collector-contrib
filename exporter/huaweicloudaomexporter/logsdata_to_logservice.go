// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudaomexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/huaweicloudaomexporter"

import (
	"encoding/json"
	"github.com/huaweicloud/huaweicloud-lts-sdk-go/producer"
	"strconv"
	"time"

	"github.com/gogo/protobuf/proto"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

const (
	ltsLogTimeUnixNano   = "timeUnixNano"
	ltsLogSeverityNumber = "severityNumber"
	ltsLogSeverityText   = "severityText"
	ltsLogContent        = "content"
	ltsLogAttribute      = "attribute"
	ltsLogFlags          = "flags"
	ltsLogResource       = "resource"
	ltsLogHost           = "host"
	ltsLogService        = "service"
	// shortcut for "otlp.instrumentation.library.name" "otlp.instrumentation.library.version"
	ltsLogInstrumentationName    = "otlp.name"
	ltsLogInstrumentationVersion = "otlp.version"
)

func logDataToLogService(ld plog.Logs) []*ExtendLog {
	var ltsLogs []*ExtendLog
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
				extendLog := mapLogRecordToLogService(logs.At(j), resourceContents, instrumentationLibraryContents)
				if extendLog != nil {
					ltsLogs = append(ltsLogs, extendLog)
				}
			}
		}
	}

	return ltsLogs
}

func resourceToLogContents(resource pcommon.Resource) []*producer.LogTag {
	logContents := make([]*producer.LogTag, 3)
	attrs := resource.Attributes()
	if hostName, ok := attrs.Get(conventions.AttributeHostName); ok {
		logContents[0] = &producer.LogTag{
			Key:   proto.String(ltsLogHost),
			Value: proto.String(hostName.AsString()),
		}
	} else {
		logContents[0] = &producer.LogTag{
			Key:   proto.String(ltsLogHost),
			Value: proto.String(""),
		}
	}

	if serviceName, ok := attrs.Get(conventions.AttributeServiceName); ok {
		logContents[1] = &producer.LogTag{
			Key:   proto.String(ltsLogService),
			Value: proto.String(serviceName.AsString()),
		}
	} else {
		logContents[1] = &producer.LogTag{
			Key:   proto.String(ltsLogService),
			Value: proto.String(""),
		}
	}

	fields := map[string]any{}
	attrs.Range(func(k string, v pcommon.Value) bool {
		if k == conventions.AttributeServiceName || k == conventions.AttributeHostName {
			return true
		}
		fields[k] = v.AsString()
		return true
	})
	attributeBuffer, _ := json.Marshal(fields)
	logContents[2] = &producer.LogTag{
		Key:   proto.String(ltsLogResource),
		Value: proto.String(string(attributeBuffer)),
	}

	return logContents
}

func instrumentationScopeToLogContents(instrumentationScope pcommon.InstrumentationScope) []*producer.LogTag {
	logContents := make([]*producer.LogTag, 2)
	logContents[0] = &producer.LogTag{
		Key:   proto.String(ltsLogInstrumentationName),
		Value: proto.String(instrumentationScope.Name()),
	}
	logContents[1] = &producer.LogTag{
		Key:   proto.String(ltsLogInstrumentationVersion),
		Value: proto.String(instrumentationScope.Version()),
	}
	return logContents
}

func mapLogRecordToLogService(lr plog.LogRecord,
	resourceContents []*producer.LogTag,
	instrumentationLibraryContents []*producer.LogTag,
) *ExtendLog {
	if lr.Body().Type() == pcommon.ValueTypeEmpty {
		return nil
	}
	var extendLog ExtendLog

	// pre alloc, refine if logContent's len > 16
	preAllocCount := 16
	extendLog.Extends = make([]*producer.LogTag, 0, preAllocCount+len(resourceContents)+len(instrumentationLibraryContents))
	contentsBuffer := make([]producer.LogTag, 0, preAllocCount)

	extendLog.Extends = append(extendLog.Extends, resourceContents...)
	extendLog.Extends = append(extendLog.Extends, instrumentationLibraryContents...)

	contentsBuffer = append(contentsBuffer, producer.LogTag{
		Key:   proto.String(ltsLogTimeUnixNano),
		Value: proto.String(strconv.FormatUint(uint64(lr.Timestamp()), 10)),
	})

	contentsBuffer = append(contentsBuffer, producer.LogTag{
		Key:   proto.String(ltsLogSeverityNumber),
		Value: proto.String(strconv.FormatInt(int64(lr.SeverityNumber()), 10)),
	})

	contentsBuffer = append(contentsBuffer, producer.LogTag{
		Key:   proto.String(ltsLogSeverityText),
		Value: proto.String(lr.SeverityText()),
	})

	fields := map[string]any{}
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		fields[k] = v.AsString()
		return true
	})
	attributeBuffer, _ := json.Marshal(fields)
	contentsBuffer = append(contentsBuffer, producer.LogTag{
		Key:   proto.String(ltsLogAttribute),
		Value: proto.String(string(attributeBuffer)),
	})

	contentsBuffer = append(contentsBuffer, producer.LogTag{
		Key:   proto.String(ltsLogContent),
		Value: proto.String(lr.Body().AsString()),
	})

	contentsBuffer = append(contentsBuffer, producer.LogTag{
		Key:   proto.String(ltsLogFlags),
		Value: proto.String(strconv.FormatUint(uint64(lr.Flags()), 16)),
	})

	contentsBuffer = append(contentsBuffer, producer.LogTag{
		Key:   proto.String(traceIDField),
		Value: proto.String(traceutil.TraceIDToHexOrEmptyString(lr.TraceID())),
	})

	contentsBuffer = append(contentsBuffer, producer.LogTag{
		Key:   proto.String(spanIDField),
		Value: proto.String(traceutil.SpanIDToHexOrEmptyString(lr.SpanID())),
	})

	for i := range contentsBuffer {
		extendLog.Extends = append(extendLog.Extends, &contentsBuffer[i])
	}

	if lr.Timestamp() > 0 {
		// convert time nano to time seconds
		extendLog.Time = proto.Uint32(uint32(lr.Timestamp() / 1000000000))
	} else {
		extendLog.Time = proto.Uint32(uint32(time.Now().Unix()))
	}

	return &extendLog
}
