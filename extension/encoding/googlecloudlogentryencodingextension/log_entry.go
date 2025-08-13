// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudlogentryencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension"
import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	gojson "github.com/goccy/go-json"
	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	ltype "google.golang.org/genproto/googleapis/logging/type"
)

const (
	gcpProjectField        = "gcp.project"
	gcpOrganizationField   = "gcp.organization"
	gcpBillingAccountField = "gcp.billing_account"
	gcpFolderField         = "gcp.folder"
	gcpResourceTypeField   = "gcp.resource_type"

	gcpOperationIDField       = "gcp.operation.id"
	gcpOperationProducerField = "gcp.operation.producer"
	gcpOperationFirstField    = "gcp.operation.first"
	gcpOperationLast          = "gcp.operation.last"

	gcpCacheLookupField                   = "gcp.cache.lookup"
	gcpCacheHitField                      = "gcp.cache.hit"
	gcpCacheValidatedWithOriginSeverField = "gcp.cache.validated_with_origin_server"
	gcpCacheFillBytes                     = "gcp.cache.fill_bytes"

	refererHeaderField         = "http.request.header.referer"
	requestServerDurationField = "http.request.server.duration"

	gcpSplitUIDField   = "gcp.split.uid"
	gcpSplitIndexField = "gcp.split.index"
	gcpSplitTotalField = "gcp.split.total"

	gcpErrorGroupField = "gcp.error_group"

	gcpAppHubPrefix                       = "gcp.apphub"
	gcpAppHubDestinationPrefix            = "gcp.apphub_destination"
	gcpAppHubApplicationContainerField    = "application.container"
	gcpAppHubApplicationLocationField     = "application.location"
	gcpAppHubApplicationIDField           = "application.id"
	gcpAppHubServiceIDField               = "service.id"
	gcpAppHubServiceEnvironmentTypeField  = "service.environment_type"
	gcpAppHubServiceCriticalityTypeField  = "service.criticality_type"
	gcpAppHubWorkloadIDField              = "workload.id"
	gcpAppHubWorkloadEnvironmentTypeField = "workload.environment_type"
	gcpAppHubWorkloadCriticalityTypeField = "workload.criticality_type"
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
	Workload *struct {
		ID              string `json:"id"`
		EnvironmentType string `json:"environmentType"`
		CriticalityType string `json:"criticalityType"`
	} `json:"workload"`
}

func strToInt(numberStr string) (int64, error) {
	num, err := strconv.ParseInt(numberStr, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to convert string %q to int64", numberStr)
	}
	return num, nil
}

func addStrAsInt(s, field string, attributes pcommon.Map) error {
	if s == "" {
		return nil
	}
	n, err := strToInt(s)
	if err != nil {
		return err
	}
	attributes.PutInt(field, n)
	return nil
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

// handleHTTPRequestField will place the HTTP attributes in the log record
func handleHTTPRequestField(attributes pcommon.Map, req *httpRequest) error {
	if req == nil {
		return nil
	}

	if err := addStrAsInt(req.ResponseSize, string(semconv.HTTPResponseSizeKey), attributes); err != nil {
		return fmt.Errorf("failed to add response size: %w", err)
	}

	if err := addStrAsInt(req.RequestSize, string(semconv.HTTPRequestSizeKey), attributes); err != nil {
		return fmt.Errorf("failed to add request size: %w", err)
	}

	if err := addStrAsInt(req.CacheFillBytes, gcpCacheFillBytes, attributes); err != nil {
		return fmt.Errorf("failed to add cache fill bytes: %w", err)
	}

	if req.Latency != "" {
		sec, after, found := strings.Cut(req.Latency, "s")
		if after != "" || !found {
			return fmt.Errorf(`invalid latency format: %q must end with "s" (e.g., "0.5s")`, req.Latency)
		}
		latency, err := strconv.ParseFloat(sec, 64)
		if err != nil {
			return fmt.Errorf(
				`invalid latency value: %q must be a number followed by "s" (e.g., "200s"), parsing error: %w`,
				req.Latency, err,
			)
		}
		attributes.PutDouble(requestServerDurationField, latency)
	}

	if req.RequestURL != "" {
		attributes.PutStr(string(semconv.URLFullKey), req.RequestURL)
		u, err := url.Parse(req.RequestURL)
		if err != nil {
			return fmt.Errorf("failed to parse request url %q: %w", req.RequestURL, err)
		}
		putStr(attributes, string(semconv.URLPathKey), u.Path)
		putStr(attributes, string(semconv.URLQueryKey), u.RawQuery)
		putStr(attributes, string(semconv.URLDomainKey), u.Host)
	}

	if req.Protocol != "" {
		if strings.Count(req.Protocol, "/") != 1 {
			return fmt.Errorf(
				`invalid protocol %q: expected exactly one "/" (format "<name>/<version>", e.g. "HTTP/1.1")`,
				req.Protocol,
			)
		}
		name, version, found := strings.Cut(req.Protocol, "/")
		if !found || name == "" || version == "" {
			return fmt.Errorf(
				`invalid protocol %q: name or version is missing (expected format "<name>/<version>", e.g. "HTTP/1.1")`,
				req.Protocol,
			)
		}
		attributes.PutStr(string(semconv.NetworkProtocolNameKey), strings.ToLower(name))
		attributes.PutStr(string(semconv.NetworkProtocolVersionKey), version)
	}

	putInt(attributes, string(semconv.HTTPResponseStatusCodeKey), req.Status)
	putStr(attributes, string(semconv.HTTPRequestMethodKey), req.RequestMethod)
	putStr(attributes, string(semconv.UserAgentOriginalKey), req.UserAgent)
	putStr(attributes, string(semconv.ClientAddressKey), req.RemoteIP)
	putStr(attributes, string(semconv.ServerAddressKey), req.ServerIP)
	putStr(attributes, refererHeaderField, req.Referer)
	putBool(attributes, gcpCacheLookupField, req.CacheLookup)
	putBool(attributes, gcpCacheHitField, req.CacheHit)
	putBool(attributes, gcpCacheValidatedWithOriginSeverField, req.CacheValidatedWithOriginServer)
	return nil
}

// handleOperationField will place the operation attributes in the log record
func handleOperationField(attributes pcommon.Map, op *operation) {
	if op == nil {
		return
	}

	putStr(attributes, gcpOperationIDField, op.ID)
	putStr(attributes, gcpOperationProducerField, op.Producer)
	putBool(attributes, gcpOperationFirstField, op.First)
	putBool(attributes, gcpOperationLast, op.Last)
}

// handleSourceLocationField will place the source location attributes in the log record
func handleSourceLocationField(attributes pcommon.Map, sourceLoc *sourceLocation) error {
	if sourceLoc == nil {
		return nil
	}

	if err := addStrAsInt(sourceLoc.Line, string(semconv.CodeLineNumberKey), attributes); err != nil {
		return fmt.Errorf("expected source location line %q to be a number: %w", sourceLoc.Line, err)
	}
	putStr(attributes, string(semconv.CodeFilePathKey), sourceLoc.File)
	putStr(attributes, string(semconv.CodeFunctionNameKey), sourceLoc.Function)
	return nil
}

// handleSplitField will place the split attributes in the log record
func handleSplitField(attributes pcommon.Map, s *split) {
	if s == nil {
		return
	}

	putStr(attributes, gcpSplitUIDField, s.UID)
	putInt(attributes, gcpSplitIndexField, s.Index)
	putInt(attributes, gcpSplitTotalField, s.TotalSplits)
}

// handleErrorGroupField will place all ids of the error group in a new log record attribute
func handleErrorGroupField(attributes pcommon.Map, errGroup []errorGroup) {
	if len(errGroup) == 0 {
		return
	}

	errorGroupSlice := attributes.PutEmptySlice(gcpErrorGroupField)
	for _, err := range errGroup {
		obj := errorGroupSlice.AppendEmpty()
		m := obj.SetEmptyMap()
		m.PutStr("id", err.ID)
	}
}

func handleAppHubField(attributes pcommon.Map, appHub *appHub, prefix string) {
	if appHub == nil {
		return
	}

	addAppHubAttr := func(field, value string) {
		field = prefix + "." + field
		putStr(attributes, field, value)
	}

	if application := appHub.Application; application != nil {
		addAppHubAttr(gcpAppHubApplicationContainerField, application.Container)
		addAppHubAttr(gcpAppHubApplicationLocationField, application.Location)
		addAppHubAttr(gcpAppHubApplicationIDField, application.ID)
	}

	if service := appHub.Service; service != nil {
		addAppHubAttr(gcpAppHubServiceEnvironmentTypeField, service.EnvironmentType)
		addAppHubAttr(gcpAppHubServiceCriticalityTypeField, service.CriticalityType)
		addAppHubAttr(gcpAppHubServiceIDField, service.ID)
	}

	if workload := appHub.Workload; workload != nil {
		addAppHubAttr(gcpAppHubWorkloadEnvironmentTypeField, workload.EnvironmentType)
		addAppHubAttr(gcpAppHubWorkloadCriticalityTypeField, workload.CriticalityType)
		addAppHubAttr(gcpAppHubWorkloadIDField, workload.ID)
	}
}

// getTraceID will parse the given trace and return the decoding id
func getTraceID(trace string) ([16]byte, error) {
	// Format: projects/my-gcp-project/traces/4ebc71f1def9274798cac4e8960d0095
	_, trace, found := strings.Cut(trace, "/traces/")
	if !found || trace == "" {
		return [16]byte{}, fmt.Errorf(`expected trace format to be "projects/<id>/traces/<id>" but got %q`, trace)
	}

	decoded, err := hex.DecodeString(trace)
	if err != nil {
		return [16]byte{}, fmt.Errorf("failed to decode trace id to hexadecimal string: %w", err)
	}
	if len(decoded) != 16 {
		return [16]byte{}, fmt.Errorf("expected trace ID hex length to be 16, got %d", len(decoded))
	}
	return [16]byte(decoded), nil
}

// getTraceID will return the decoded span id
func getSpanID(spanIDStr string) ([8]byte, error) {
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

// getSeverityNumber will map the severity to the plog.SeverityNumber
func getSeverityNumber(severity string) plog.SeverityNumber {
	// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogSeverity
	switch severity {
	case ltype.LogSeverity_DEBUG.String():
		return plog.SeverityNumberDebug
	case ltype.LogSeverity_INFO.String():
		return plog.SeverityNumberInfo
	case ltype.LogSeverity_NOTICE.String():
		return plog.SeverityNumberInfo2
	case ltype.LogSeverity_WARNING.String():
		return plog.SeverityNumberWarn
	case ltype.LogSeverity_ERROR.String():
		return plog.SeverityNumberError
	case ltype.LogSeverity_CRITICAL.String():
		return plog.SeverityNumberFatal
	case ltype.LogSeverity_ALERT.String():
		return plog.SeverityNumberFatal2
	case ltype.LogSeverity_EMERGENCY.String():
		return plog.SeverityNumberFatal4
	case ltype.LogSeverity_DEFAULT.String():
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

// handleLogNameField will parse the logName and fill the expected attributes
func handleLogNameField(logName string, resourceAttr pcommon.Map) error {
	if logName == "" {
		return nil
	}

	// logName has one of the following formats:
	// "projects/[PROJECT_ID]/logs/[LOG_ID]"
	// "organizations/[ORGANIZATION_ID]/logs/[LOG_ID]"
	// "billingAccounts/[BILLING_ACCOUNT_ID]/logs/[LOG_ID]"
	// "folders/[FOLDER_ID]/logs/[LOG_ID]"
	addIDsAttributes := func(prefix, format, field string) error {
		_, rest, _ := strings.Cut(logName, prefix)
		id, cloudID, _ := strings.Cut(rest, "/logs/")
		if cloudID == "" || id == "" {
			return fmt.Errorf(
				`expected log name %q to have format "%s/%s/logs/[LOG_ID]"`, logName, prefix, format,
			)
		}
		resourceAttr.PutStr(field, id)
		resourceAttr.PutStr(string(semconv.CloudResourceIDKey), cloudID)
		return nil
	}

	switch {
	case strings.HasPrefix(logName, "projects/"):
		return addIDsAttributes("projects/", "[PROJECT_ID]", gcpProjectField)
	case strings.HasPrefix(logName, "organizations/"):
		return addIDsAttributes("organizations/", "[ORGANIZATION_ID]", gcpOrganizationField)
	case strings.HasPrefix(logName, "billingAccounts/"):
		return addIDsAttributes("billingAccounts/", "[BILLING_ACCOUNT_ID]", gcpBillingAccountField)
	case strings.HasPrefix(logName, "folders/"):
		return addIDsAttributes("folders/", "[FOLDER_ID]/", gcpFolderField)
	default:
		return fmt.Errorf("unrecognized log name %q", logName)
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

	putStr(logRecord.Attributes(), string(semconv.LogRecordUIDKey), log.InsertID)

	if err := handleLogNameField(log.LogName, resourceAttributes); err != nil {
		return fmt.Errorf("failed to handle log name field: %w", err)
	}

	if err := handleHTTPRequestField(logRecord.Attributes(), log.HTTPRequest); err != nil {
		return fmt.Errorf("failed to handle HTTP request entry field: %w", err)
	}

	if err := handleSourceLocationField(logRecord.Attributes(), log.SourceLocation); err != nil {
		return fmt.Errorf("failed to handle source location entry field: %w", err)
	}

	if log.Resource != nil {
		resourceAttributes.PutStr(gcpResourceTypeField, log.Resource.Type)
		for k, v := range log.Resource.Labels {
			resourceAttributes.PutStr(strcase.ToSnakeWithIgnore(fmt.Sprintf("gcp.label.%v", k), "."), v)
		}
	}

	if log.Severity != "" {
		logRecord.SetSeverityText(log.Severity)
		logRecord.SetSeverityNumber(getSeverityNumber(log.Severity))
	}

	if log.Trace != "" {
		traceIDBytes, err := getTraceID(log.Trace)
		if err != nil {
			return err
		}
		logRecord.SetTraceID(traceIDBytes)
	}

	if log.SpanID != "" {
		spanIDBytes, err := getSpanID(log.SpanID)
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
		logRecord.Attributes().PutStr(strcase.ToSnakeWithIgnore(fmt.Sprintf("gcp.label.%v", k), "."), v)
	}

	handleOperationField(logRecord.Attributes(), log.Operation)
	handleSplitField(logRecord.Attributes(), log.Split)
	handleErrorGroupField(logRecord.Attributes(), log.ErrorGroups)

	handleAppHubField(logRecord.Attributes(), log.AppHub, gcpAppHubPrefix)
	handleAppHubField(logRecord.Attributes(), log.AppHubDestination, gcpAppHubDestinationPrefix)

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
