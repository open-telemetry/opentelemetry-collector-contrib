// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"
import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	gojson "github.com/goccy/go-json"
	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// See: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry
type logEntry struct {
	ProtoPayload gojson.RawMessage `json:"protoPayload"`
	TextPayload  string            `json:"textPayload"`
	JSONPayload  gojson.RawMessage `json:"jsonPayload"`

	ReceiveTimestamp *time.Time `json:"receiveTimestamp"`
	Timestamp        *time.Time `json:"timestamp"`

	InsertID     string            `json:"insertId"`
	LogName      string            `json:"logName"`
	Severity     string            `json:"severity"`
	Trace        string            `json:"trace"`
	SpanID       string            `json:"spanId"`
	TraceSampled *bool             `json:"traceSampled"`
	Labels       map[string]string `json:"labels"`

	HTTPRequest *httpRequest `json:"httpRequest"`

	Resource *struct {
		Type   string            `json:"type"`
		Labels map[string]string `json:"labels"`
	} `json:"resource"`

	Operation *operation `json:"operation"`

	SourceLocation *sourceLocation `json:"sourceLocation"`

	Split *split `json:"split"`

	ErrorGroups []errorGroup `json:"errorGroups"`

	AppHub *appHub `json:"apphub"`

	AppHubDestination *appHub `json:"apphubDestination"`
}

type errorGroup struct {
	ID string `json:"id"`
}

type split struct {
	UID         string `json:"uid"`
	Index       *int64 `json:"index"`
	TotalSplits *int64 `json:"totalSplits"`
}

type sourceLocation struct {
	File     string `json:"file"`
	Line     string `json:"line"`
	Function string `json:"function"`
}
type operation struct {
	ID       string `json:"id"`
	Producer string `json:"producer"`
	First    *bool  `json:"first"`
	Last     *bool  `json:"last"`
}

type httpRequest struct {
	RequestMethod                  string `json:"requestMethod"`
	RequestURL                     string `json:"requestURL"`
	RequestSize                    string `json:"requestSize"`
	Status                         *int64 `json:"status"`
	ResponseSize                   string `json:"responseSize"`
	UserAgent                      string `json:"userAgent"`
	RemoteIP                       string `json:"remoteIP"`
	ServerIP                       string `json:"serverIP"`
	Referer                        string `json:"referer"`
	Latency                        string `json:"latency"`
	CacheLookup                    *bool  `json:"cacheLookup"`
	CacheHit                       *bool  `json:"cacheHit"`
	CacheValidatedWithOriginServer *bool  `json:"cacheValidatedWithOriginServer"`
	CacheFillBytes                 string `json:"cacheFillBytes"`
	Protocol                       string `json:"protocol"`
}

type appHub struct {
	Application *struct {
		Container string `json:"container"`
		Location  string `json:"location"`
		ID        string `json:"id"`
	} `json:"application"`
	Service *struct {
		ID              string `json:"id"`
		EnvironmentType string `json:"environmentType"`
		CriticalityType string `json:"criticalityType"`
	} `json:"service"`
	Workflow *struct {
		ID              string `json:"id"`
		EnvironmentType string `json:"environmentType"`
		CriticalityType string `json:"criticalityType"`
	} `json:"workload"`
}

func strToInt(numberStr string) (*int64, error) {
	num, err := strconv.ParseInt(numberStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to convert string %q to int64", numberStr)
	}
	return &num, nil
}

func putStr(attr pcommon.Map, field, value string) {
	if value != "" {
		attr.PutStr(field, value)
	}
}

func putInt(attr pcommon.Map, field string, value *int64) {
	if value != nil {
		attr.PutInt(field, *value)
	}
}

func putBool(attr pcommon.Map, field string, value *bool) {
	if value != nil {
		attr.PutBool(field, *value)
	}
}

func handleHTTPRequestField(logRecord plog.LogRecord, req *httpRequest) {
	if req == nil {
		return
	}

	requestMap := logRecord.Attributes().PutEmptyMap("gcp.http_request")
	if n, err := strToInt(req.ResponseSize); err == nil {
		putInt(requestMap, "response_size", n)
	}
	if n, err := strToInt(req.RequestSize); err == nil {
		putInt(requestMap, "request_size", n)
	}
	putStr(requestMap, "request_method", req.RequestMethod)
	putStr(requestMap, "request_url", req.RequestURL)
	putInt(requestMap, "status", req.Status)
	putStr(requestMap, "user_agent", req.UserAgent)
	putStr(requestMap, "remote_ip", req.RemoteIP)
	putStr(requestMap, "server_ip", req.ServerIP)
	putStr(requestMap, "referer", req.Referer)
	putStr(requestMap, "latency", req.Latency)
	putStr(requestMap, "protocol", req.Protocol)
	putStr(requestMap, "cache_fill_bytes", req.CacheFillBytes)
	putBool(requestMap, "cache_lookup", req.CacheLookup)
	putBool(requestMap, "cache_hit", req.CacheHit)
	putBool(requestMap, "cache_validated_with_origin_server", req.CacheValidatedWithOriginServer)
}

func handleOperationField(logRecord plog.LogRecord, op *operation) {
	if op == nil {
		return
	}

	operationMap := logRecord.Attributes().PutEmptyMap("gcp.origin")
	putStr(operationMap, "id", op.ID)
	putStr(operationMap, "producer", op.Producer)
	putBool(operationMap, "first", op.First)
	putBool(operationMap, "last", op.Last)
}

func handleSourceLocationField(logRecord plog.LogRecord, sourceLoc *sourceLocation) {
	if sourceLoc == nil {
		return
	}

	sourceLocMap := logRecord.Attributes().PutEmptyMap("gcp.source_location")
	putStr(sourceLocMap, "file", sourceLoc.File)
	putStr(sourceLocMap, "line", sourceLoc.Line)
	putStr(sourceLocMap, "function", sourceLoc.Function)
}

func handleSplitField(logRecord plog.LogRecord, s *split) {
	if s == nil {
		return
	}

	splitMap := logRecord.Attributes().PutEmptyMap("gcp.split")
	putStr(splitMap, "uid", s.UID)
	putInt(splitMap, "index", s.Index)
	putInt(splitMap, "total_splits", s.TotalSplits)
}

func handleErrorGroupField(logRecord plog.LogRecord, errGroup []errorGroup) {
	if len(errGroup) == 0 {
		return
	}
	errorGroupSlice := logRecord.Attributes().PutEmptySlice("gcp.error_group")
	for _, err := range errGroup {
		errorGroupSlice.AppendEmpty().SetStr(err.ID)
	}
}

func handleAppHubField(logRecord plog.LogRecord, appHub *appHub, mapName string) {
	if appHub == nil {
		return
	}

	m := pcommon.NewMap()

	if application := appHub.Application; application != nil {
		mApplication := m.PutEmptyMap("application")
		putStr(mApplication, "container", application.Container)
		putStr(mApplication, "location", application.Location)
		putStr(mApplication, "id", application.ID)
	}

	if service := appHub.Service; service != nil {
		mService := m.PutEmptyMap("service")
		putStr(mService, "environment_type", service.EnvironmentType)
		putStr(mService, "criticality_type", service.CriticalityType)
		putStr(mService, "id", service.ID)
	}

	if workflow := appHub.Workflow; workflow != nil {
		mWorkflow := m.PutEmptyMap("workflow")
		putStr(mWorkflow, "environment_type", workflow.EnvironmentType)
		putStr(mWorkflow, "criticality_type", workflow.CriticalityType)
		putStr(mWorkflow, "id", workflow.ID)
	}

	if m.Len() > 0 {
		m.CopyTo(logRecord.Attributes().PutEmptyMap(mapName))
	}
}

func cloudLoggingTraceToTraceIDBytes(trace string) ([16]byte, error) {
	// Format: projects/my-gcp-project/traces/4ebc71f1def9274798cac4e8960d0095
	lastSlashIdx := strings.LastIndex(trace, "/")
	if lastSlashIdx == -1 {
		return [16]byte{}, fmt.Errorf(
			"invalid trace format: expected 'projects/{project}/traces/{trace_id}' but got %q", trace,
		)
	}
	traceIDStr := trace[lastSlashIdx+1:]

	return traceIDStrToTraceIDBytes(traceIDStr)
}

func traceIDStrToTraceIDBytes(traceIDStr string) ([16]byte, error) {
	decoded, err := hex.DecodeString(traceIDStr)
	if err != nil {
		return [16]byte{}, fmt.Errorf("failed to decode trace id to hexadecimal string: %w", err)
	}
	if len(decoded) != 16 {
		return [16]byte{}, fmt.Errorf("expected trace ID hex length to be 16, got %d", len(decoded))
	}
	return [16]byte(decoded), nil
}

func spanIDStrToSpanIDBytes(spanIDStr string) ([8]byte, error) {
	// TODO cloud Run sends invalid span id's, make sure we're not crashing,
	// see https://issuetracker.google.com/issues/338634230?pli=1

	decoded, err := hex.DecodeString(spanIDStr)
	if err != nil {
		return [8]byte{}, fmt.Errorf("failed to decode span id to hexadecimal string: %w", err)
	}
	if len(decoded) != 8 {
		return [8]byte{}, fmt.Errorf("expected span ID hex length to be 8, got %d", len(decoded))
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

func setBodyFromJSON(logRecord plog.LogRecord, value gojson.RawMessage) error {
	// {json,proto,text}_payload -> Body
	var payload any
	err := gojson.Unmarshal(value, &payload)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON payload: %w", err)
	}
	// Note: json.Unmarshal will turn a bare string into a
	// go string, so this call will correctly set the body
	// to a string Value.
	_ = logRecord.Body().FromRaw(payload)
	return nil
}

func setBodyFromText(logRecord plog.LogRecord, value string) {
	logRecord.Body().SetStr(value)
}

func handleTextPayloadField(logRecord plog.LogRecord, value string) {
	setBodyFromText(logRecord, value)
}

func handleJSONPayloadField(logRecord plog.LogRecord, value gojson.RawMessage, config Config) error {
	switch config.HandleJSONPayloadAs {
	case HandleAsJSON:
		return setBodyFromJSON(logRecord, value)
	case HandleAsText:
		setBodyFromText(logRecord, string(value))
		return nil
	default:
		return errors.New("unrecognized JSON payload type")
	}
}

func handleProtoPayloadField(logRecord plog.LogRecord, value gojson.RawMessage, config Config) error {
	switch config.HandleProtoPayloadAs {
	case HandleAsJSON:
		return setBodyFromJSON(logRecord, value)
	case HandleAsProtobuf:
		return setBodyFromProto(logRecord, value)
	case HandleAsText:
		setBodyFromText(logRecord, string(value))
		return nil
	default:
		return errors.New("unrecognized proto payload type")
	}
}

// handleLogEntryFields will place each entry of logEntry as either an attribute of the log,
// or as part of the log body, in case of payload.
func handleLogEntryFields(resourceAttributes pcommon.Map, logRecord plog.LogRecord, log logEntry, cfg Config) error {
	if ts := log.Timestamp; ts != nil {
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(*ts))
	}
	if ts := log.ReceiveTimestamp; ts != nil {
		logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(*ts))
	}

	putStr(logRecord.Attributes(), "log.record.uid", log.InsertID)
	putStr(logRecord.Attributes(), "gcp.log_name", log.LogName)

	if log.Resource != nil {
		resourceAttributes.PutStr("gcp.resource_type", log.Resource.Type)
		for k, v := range log.Resource.Labels {
			resourceAttributes.PutStr(strcase.ToSnakeWithIgnore(fmt.Sprintf("gcp.%v", k), "."), v)
		}
	}

	if log.Severity != "" {
		logRecord.SetSeverityText(log.Severity)
		logRecord.SetSeverityNumber(cloudLoggingSeverityToNumber(log.Severity))
	}

	if log.Trace != "" {
		traceIDBytes, err := cloudLoggingTraceToTraceIDBytes(log.Trace)
		if err != nil {
			return err
		}
		logRecord.SetTraceID(traceIDBytes)
	}

	if log.SpanID != "" {
		spanIDBytes, err := spanIDStrToSpanIDBytes(log.SpanID)
		if err != nil {
			return err
		}
		logRecord.SetSpanID(spanIDBytes)
	}

	if log.TraceSampled != nil {
		var flags plog.LogRecordFlags
		logRecord.SetFlags(flags.WithIsSampled(*log.TraceSampled))
	}

	for k, v := range log.Labels {
		logRecord.Attributes().PutStr(strcase.ToSnakeWithIgnore(fmt.Sprintf("gcp.%v", k), "."), v)
	}

	handleHTTPRequestField(logRecord, log.HTTPRequest)
	handleOperationField(logRecord, log.Operation)
	handleSourceLocationField(logRecord, log.SourceLocation)
	handleSplitField(logRecord, log.Split)
	handleErrorGroupField(logRecord, log.ErrorGroups)

	handleAppHubField(logRecord, log.AppHub, "apphub")
	handleAppHubField(logRecord, log.AppHubDestination, "apphub_destination")

	if len(log.ProtoPayload) > 0 {
		if err := handleProtoPayloadField(logRecord, log.ProtoPayload, cfg); err != nil {
			return fmt.Errorf("failed to handle proto payload field: %w", err)
		}
	}
	if len(log.JSONPayload) > 0 {
		if err := handleJSONPayloadField(logRecord, log.JSONPayload, cfg); err != nil {
			return fmt.Errorf("failed to handle json payload field: %w", err)
		}
	}
	if log.TextPayload != "" {
		handleTextPayloadField(logRecord, log.TextPayload)
	}

	return nil
}
