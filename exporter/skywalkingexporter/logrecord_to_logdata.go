// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/skywalkingexporter"

import (
	"encoding/hex"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	logpb "skywalking.apache.org/repo/goapi/collect/logging/v3"
)

const (
	spanIDField            = "spanID"
	severityNumber         = "severityNumber"
	severityText           = "severityText"
	flags                  = "flags"
	instrumentationName    = "otlp.name"
	instrumentationVersion = "otlp.version"
	defaultServiceName     = "otel-collector"
)

func logRecordToLogData(ld plog.Logs) []*logpb.LogData {
	var lds []*logpb.LogData
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		ills := rl.ScopeLogs()
		resource := rl.Resource()
		for j := 0; j < ills.Len(); j++ {
			ils := ills.At(j)
			logs := ils.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				logData := &logpb.LogData{}
				logData.Tags = &logpb.LogTags{}
				resourceToLogData(resource, logData)
				instrumentationLibraryToLogData(ils.Scope(), logData)
				mapLogRecordToLogData(logs.At(k), logData)
				lds = append(lds, logData)
			}
		}
	}
	return lds
}

func resourceToLogData(resource pcommon.Resource, logData *logpb.LogData) {
	attrs := resource.Attributes()

	if serviceName, ok := attrs.Get(conventions.AttributeServiceName); ok {
		logData.Service = serviceName.AsString()
	} else {
		logData.Service = defaultServiceName
	}

	if serviceInstanceID, ok := attrs.Get(conventions.AttributeServiceInstanceID); ok {
		logData.ServiceInstance = serviceInstanceID.AsString()
	}

	attrs.Range(func(k string, v pcommon.Value) bool {
		logData.Tags.Data = append(logData.Tags.Data, &common.KeyStringValuePair{
			Key:   k,
			Value: v.AsString(),
		})
		return true
	})
}

func instrumentationLibraryToLogData(scope pcommon.InstrumentationScope, logData *logpb.LogData) {
	if nameValue := scope.Name(); nameValue != "" {
		logData.Tags.Data = append(logData.Tags.Data, &common.KeyStringValuePair{
			Key:   instrumentationName,
			Value: nameValue,
		})
	}
	if version := scope.Version(); version != "" {
		logData.Tags.Data = append(logData.Tags.Data, &common.KeyStringValuePair{
			Key:   instrumentationVersion,
			Value: version,
		})
	}
}

func mapLogRecordToLogData(lr plog.LogRecord, logData *logpb.LogData) {
	if lr.Body().Type() == pcommon.ValueTypeEmpty {
		return
	}

	if timestamp := lr.Timestamp(); timestamp > 0 {
		logData.Timestamp = lr.Timestamp().AsTime().UnixNano() / int64(time.Millisecond)
	}

	if sn := strconv.FormatInt(int64(lr.SeverityNumber()), 10); sn != "" {
		logData.Tags.Data = append(logData.Tags.Data, &common.KeyStringValuePair{
			Key:   severityNumber,
			Value: sn,
		})
	}

	if st := lr.SeverityText(); st != "" {
		logData.Tags.Data = append(logData.Tags.Data, &common.KeyStringValuePair{
			Key:   severityText,
			Value: st,
		})
	}

	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		logData.Tags.Data = append(logData.Tags.Data, &common.KeyStringValuePair{
			Key:   k,
			Value: v.AsString(),
		})
		return true
	})

	logData.Body = &logpb.LogDataBody{
		Type: "body-type",
		Content: &logpb.LogDataBody_Text{
			Text: &logpb.TextLog{
				Text: lr.Body().AsString(),
			}},
	}

	if flag := strconv.FormatUint(uint64(lr.Flags()), 16); flag != "" {
		logData.Tags.Data = append(logData.Tags.Data, &common.KeyStringValuePair{
			Key:   flags,
			Value: flag,
		})
	}

	if traceID := lr.TraceID(); !traceID.IsEmpty() {
		logData.TraceContext = &logpb.TraceContext{TraceId: hex.EncodeToString(traceID[:])}
	}

	if spanID := lr.SpanID(); !spanID.IsEmpty() {
		logData.Tags.Data = append(logData.Tags.Data, &common.KeyStringValuePair{
			Key:   spanIDField,
			Value: hex.EncodeToString(spanID[:]),
		})
	}
}
