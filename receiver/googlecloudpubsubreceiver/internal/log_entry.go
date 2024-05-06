// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal"

import (
	"context"
	"encoding/hex"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/logging/apiv2/loggingpb"
	"github.com/iancoleman/strcase"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var invalidTraceID = [16]byte{}
var invalidSpanID = [8]byte{}
var errorParsingLogItem = errors.New("error parsing log item")

func cloudLoggingTraceToTraceIDBytes(trace string) ([16]byte, error) {
	// Format: projects/my-gcp-project/traces/4ebc71f1def9274798cac4e8960d0095
	lastSlashIdx := strings.LastIndex(trace, "/")
	if lastSlashIdx == -1 {
		return invalidTraceID, errorParsingLogItem
	}
	traceIDStr := trace[lastSlashIdx+1:]

	return traceIDStrToTraceIDBytes(traceIDStr)
}

func traceIDStrToTraceIDBytes(traceIDStr string) ([16]byte, error) {
	decoded, err := hex.DecodeString(traceIDStr)
	if err != nil {
		return invalidTraceID, err
	}
	if len(decoded) != 16 {
		return invalidTraceID, errorParsingLogItem
	}
	return [16]byte(decoded), nil
}

func spanIDStrToSpanIDBytes(spanIDStr string) ([8]byte, error) {
	decoded, err := hex.DecodeString(spanIDStr)
	if err != nil {
		return invalidSpanID, err
	}
	if len(decoded) != 8 {
		return invalidSpanID, errorParsingLogItem
	}
	return [8]byte(decoded), nil
}

func cloudLoggingSeverityToNumber(severity string) plog.SeverityNumber {
	// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogSeverity
	switch severity {
	case "DEBUG":
		return plog.SeverityNumberDebug
	case "INFO":
		return plog.SeverityNumberInfo
	case "NOTICE":
		return plog.SeverityNumberInfo2
	case "WARNING":
		return plog.SeverityNumberWarn
	case "ERROR":
		return plog.SeverityNumberError
	case "CRITICAL":
		return plog.SeverityNumberFatal
	case "ALERT":
		return plog.SeverityNumberFatal2
	case "EMERGENCY":
		return plog.SeverityNumberFatal4
	case "DEFAULT":
	}
	return plog.SeverityNumberUnspecified
}

var desc protoreflect.MessageDescriptor
var descOnce sync.Once

func getLogEntryDescriptor() protoreflect.MessageDescriptor {
	descOnce.Do(func() {
		var logEntry loggingpb.LogEntry

		desc = logEntry.ProtoReflect().Descriptor()
	})
	return desc
}

type extractFn func(pcommon.Map, plog.LogRecord, pcommon.Map, string, stdjson.RawMessage, Options) error

func extractFromFields(_ context.Context, source map[string]stdjson.RawMessage, resourceAttributes pcommon.Map, logRecord plog.LogRecord, attributes pcommon.Map, extractFns map[string]extractFn, options Options) {
	for key, value := range source {
		fn := extractFns[key]
		if fn != nil {
			err := fn(resourceAttributes, logRecord, attributes, key, value, options)
			if err == nil {
				delete(source, key)
			}
		}
	}
}

func handleTimestamp(_ pcommon.Map, logRecord plog.LogRecord, _ pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	// timestamp -> Timestamp
	var t time.Time
	err := json.Unmarshal(value, &t)
	if err != nil {
		return err
	}
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(t))
	return nil
}

func handleReceiveTimestamp(_ pcommon.Map, logRecord plog.LogRecord, _ pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	// receiveTimestamp -> Timestamp
	var t time.Time
	err := json.Unmarshal(value, &t)
	if err != nil {
		return err
	}
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(t))
	return nil
}

// handleInsertID extracts the insertId field from a LogEntry and puts it into the log record attributes.
// insertId -> Attributes[“log.record.uid”] stability: experimental
// see https://github.com/open-telemetry/semantic-conventions/blob/main/model/registry/log.yaml
func handleInsertID(_ pcommon.Map, _ plog.LogRecord, logAttributes pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	var insertID string
	err := json.Unmarshal(value, &insertID)
	if err != nil {
		return err
	}
	logAttributes.PutStr("log.record.uid", insertID)
	return nil
}

// handleLogName extracts the logName field from a LogEntry and puts it into the log record attributes.
// logName -> Attributes[“gcp.log_name”] stability: experimental
func handleLogName(_ pcommon.Map, _ plog.LogRecord, logAttributes pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	var logName string
	err := json.Unmarshal(value, &logName)
	if err != nil {
		return err
	}
	logAttributes.PutStr("gcp.log_name", logName)
	return nil
}

func handleResource(resourceAttributes pcommon.Map, _ plog.LogRecord, _ pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	// resource -> Resource
	// mapping type -> gcp.resource_type
	// labels -> gcp.label.<label>
	var protoRes monitoredres.MonitoredResource
	err := protojson.Unmarshal(value, &protoRes)
	if err != nil {
		return err
	}
	resourceAttributes.PutStr("gcp.resource_type", protoRes.GetType())
	for k, v := range protoRes.GetLabels() {
		resourceAttributes.PutStr(strcase.ToSnakeWithIgnore(fmt.Sprintf("gcp.%v", k), "."), v)
	}
	return nil
}

func handleTextPayload(_ pcommon.Map, logRecord plog.LogRecord, _ pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	var textPayload string
	err := json.Unmarshal(value, &textPayload)
	if err != nil {
		return err
	}
	logRecord.Body().SetStr(textPayload)
	return nil
}

func handleJSONPayload(_ pcommon.Map, logRecord plog.LogRecord, _ pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	// {json,proto,text}_payload -> Body
	var payload any
	err := json.Unmarshal(value, &payload)
	if err != nil {
		return err
	}
	// Note: json.Unmarshal will turn a bare string into a
	// go string, so this call will correctly set the body
	// to a string Value.
	_ = logRecord.Body().FromRaw(payload)
	return nil
}

func handleSeverity(_ pcommon.Map, logRecord plog.LogRecord, _ pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	var severity string
	err := json.Unmarshal(value, &severity)
	if err != nil {
		return err
	}
	// severity -> Severity
	// According to the spec, this is the original string representation of
	// the severity as it is known at the source:
	//   https://opentelemetry.io/docs/reference/specification/logs/data-model/#field-severitytext
	logRecord.SetSeverityText(severity)
	logRecord.SetSeverityNumber(cloudLoggingSeverityToNumber(severity))
	return nil
}

func handleTrace(_ pcommon.Map, logRecord plog.LogRecord, _ pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	var trace string
	err := json.Unmarshal(value, &trace)
	if err != nil {
		return err
	}
	traceIDBytes, err := cloudLoggingTraceToTraceIDBytes(trace)
	if err != nil {
		return err
	}
	logRecord.SetTraceID(traceIDBytes)
	return nil
}

func handleSpanID(_ pcommon.Map, logRecord plog.LogRecord, _ pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	var spanID string
	err := json.Unmarshal(value, &spanID)
	if err != nil {
		return err
	}
	spanIDBytes, err := spanIDStrToSpanIDBytes(spanID)
	if err != nil {
		return err
	}
	logRecord.SetSpanID(spanIDBytes)
	return nil
}

func handleTraceSampled(_ pcommon.Map, logRecord plog.LogRecord, _ pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	var traceSampled bool
	err := json.Unmarshal(value, &traceSampled)
	if err != nil {
		return err
	}
	var flags plog.LogRecordFlags
	logRecord.SetFlags(flags.WithIsSampled(traceSampled))
	return nil
}

func handleLabels(_ pcommon.Map, _ plog.LogRecord, logAttributes pcommon.Map, _ string, value stdjson.RawMessage, _ Options) error {
	var labels map[string]string
	err := json.Unmarshal(value, &labels)
	if err != nil {
		return err
	}

	// labels -> Attributes
	for k, v := range labels {
		logAttributes.PutStr(strcase.ToSnakeWithIgnore(fmt.Sprintf("gcp.%v", k), "."), v)
	}
	return nil
}

func handleHTTPRequest(_ pcommon.Map, _ plog.LogRecord, logAttributes pcommon.Map, key string, value stdjson.RawMessage, _ Options) error {
	httpRequestAttrs := logAttributes.PutEmptyMap("gcp.http_request")
	err := translateInto(httpRequestAttrs, getLogEntryDescriptor().Fields().ByJSONName(key).Message(), value, snakeifyKeys)
	if err != nil {
		return err
	}
	return nil
}

// TranslateLogEntry translates a JSON-encoded LogEntry message into a pair of
// pcommon.Resource and plog.LogRecord, trying to keep as close as possible to
// the semantic conventions.
//
// For maximum fidelity, the decoding is done according to the protobuf message
// schema; this ensures that a numeric value in the input is correctly
// translated to either an integer or a double in the output. It falls back to
// plain JSON decoding if payload type is not available in the proto registry.
func TranslateLogEntry(context context.Context, _ *zap.Logger, data []byte, options Options) (pcommon.Resource, plog.LogRecord, error) {
	lr := plog.NewLogRecord()
	res := pcommon.NewResource()

	var src map[string]stdjson.RawMessage
	err := json.Unmarshal(data, &src)

	if err != nil {
		return res, lr, err
	}

	resAttrs := res.Attributes()
	attrs := lr.Attributes()

	extractFns := map[string]extractFn{
		"protoPayload":     handleProtoPayload,
		"receiveTimestamp": handleReceiveTimestamp,
		"timestamp":        handleTimestamp,
		"insertId":         handleInsertID,
		"logName":          handleLogName,
		"textPayload":      handleTextPayload,
		"jsonPayload":      handleJSONPayload,
		"severity":         handleSeverity,
		"trace":            handleTrace,
		"spanId":           handleSpanID,
		"traceSampled":     handleTraceSampled,
		"labels":           handleLabels,
		"httpRequest":      handleHTTPRequest,
		"resource":         handleResource,
	}
	extractFromFields(context, src, resAttrs, lr, attrs, extractFns, options)

	// All other fields -> Attributes["gcp.*"]
	// At this point we cleared all the fields that have special handling;
	// translate the rest into the attributes map.
	_ = translateInto(attrs, getLogEntryDescriptor(), src, preserveDst, prefixKeys("gcp."), snakeifyKeys)

	return res, lr, nil
}
