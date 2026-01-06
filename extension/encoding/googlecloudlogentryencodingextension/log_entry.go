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
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	ltype "google.golang.org/genproto/googleapis/logging/type"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/apploadbalancerlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/auditlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/dnslog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/passthroughnlb"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/proxynlb"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/vpcflowlog"
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

// getEncodingFormat maps GCP log types to encoding format values
func getEncodingFormat(logType string) string {
	switch logType {
	case auditlog.ActivityLogNameSuffix,
		auditlog.DataAccessLogNameSuffix,
		auditlog.SystemEventLogNameSuffix,
		auditlog.PolicyLogNameSuffix:
		return constants.GCPFormatAuditLog
	case vpcflowlog.NetworkManagementNameSuffix,
		vpcflowlog.ComputeNameSuffix:
		return constants.GCPFormatVPCFlowLog
	case apploadbalancerlog.GlobalAppLoadBalancerLogSuffix,
		apploadbalancerlog.RegionalAppLoadBalancerLogSuffix:
		return constants.GCPFormatLoadBalancerLog
	case proxynlb.ConnectionsLogNameSuffix:
		return constants.GCPFormatProxyNLBLog
	case dnslog.CloudDNSQueryLogSuffix:
		return constants.GCPFormatDNSQueryLog
	case passthroughnlb.ConnectionsLogNameSuffix:
		return constants.GCPFormatPassthroughNLBLog
	default:
		return ""
	}
}

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

// handleHTTPRequestField will place the HTTP attributes in the log record
func handleHTTPRequestField(attributes pcommon.Map, req *httpRequest) error {
	if req == nil {
		return nil
	}

	if err := shared.AddStrAsInt(string(conventions.HTTPResponseSizeKey), req.ResponseSize, attributes); err != nil {
		return fmt.Errorf("failed to add response size: %w", err)
	}

	if err := shared.AddStrAsInt(string(conventions.HTTPRequestSizeKey), req.RequestSize, attributes); err != nil {
		return fmt.Errorf("failed to add request size: %w", err)
	}

	if err := shared.AddStrAsInt(gcpCacheFillBytes, req.CacheFillBytes, attributes); err != nil {
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
		attributes.PutStr(string(conventions.URLFullKey), req.RequestURL)
		u, err := url.Parse(req.RequestURL)
		if err != nil {
			return fmt.Errorf("failed to parse request url %q: %w", req.RequestURL, err)
		}
		shared.PutStr(string(conventions.URLPathKey), u.Path, attributes)
		shared.PutStr(string(conventions.URLQueryKey), u.RawQuery, attributes)
		shared.PutStr(string(conventions.URLDomainKey), u.Host, attributes)
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
		attributes.PutStr(string(conventions.NetworkProtocolNameKey), strings.ToLower(name))
		attributes.PutStr(string(conventions.NetworkProtocolVersionKey), version)
	}

	shared.PutInt(string(conventions.HTTPResponseStatusCodeKey), req.Status, attributes)
	shared.PutStr(string(conventions.HTTPRequestMethodKey), req.RequestMethod, attributes)
	shared.PutStr(string(conventions.UserAgentOriginalKey), req.UserAgent, attributes)
	shared.PutStr(string(conventions.NetworkPeerAddressKey), req.RemoteIP, attributes)
	shared.PutStr(string(conventions.ServerAddressKey), req.ServerIP, attributes)
	shared.PutStr(refererHeaderField, req.Referer, attributes)
	shared.PutBool(gcpCacheLookupField, req.CacheLookup, attributes)
	shared.PutBool(gcpCacheHitField, req.CacheHit, attributes)
	shared.PutBool(gcpCacheValidatedWithOriginSeverField, req.CacheValidatedWithOriginServer, attributes)
	return nil
}

// handleOperationField will place the operation attributes in the log record
func handleOperationField(attributes pcommon.Map, op *operation) {
	if op == nil {
		return
	}

	shared.PutStr(gcpOperationIDField, op.ID, attributes)
	shared.PutStr(gcpOperationProducerField, op.Producer, attributes)
	shared.PutBool(gcpOperationFirstField, op.First, attributes)
	shared.PutBool(gcpOperationLast, op.Last, attributes)
}

// handleSourceLocationField will place the source location attributes in the log record
func handleSourceLocationField(attributes pcommon.Map, sourceLoc *sourceLocation) error {
	if sourceLoc == nil {
		return nil
	}

	if err := shared.AddStrAsInt(string(conventions.CodeLineNumberKey), sourceLoc.Line, attributes); err != nil {
		return fmt.Errorf("expected source location line %q to be a number: %w", sourceLoc.Line, err)
	}
	shared.PutStr(string(conventions.CodeFilePathKey), sourceLoc.File, attributes)
	shared.PutStr(string(conventions.CodeFunctionNameKey), sourceLoc.Function, attributes)
	return nil
}

// handleSplitField will place the split attributes in the log record
func handleSplitField(attributes pcommon.Map, s *split) {
	if s == nil {
		return
	}

	shared.PutStr(gcpSplitUIDField, s.UID, attributes)
	shared.PutInt(gcpSplitIndexField, s.Index, attributes)
	shared.PutInt(gcpSplitTotalField, s.TotalSplits, attributes)
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
		shared.PutStr(field, value, attributes)
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

// handleLogNameField parses a GCP logName string and extracts resource identifiers and log type.
// The logName must follow one of these formats:
//   - "projects/[PROJECT_ID]/logs/[LOG_ID]"
//   - "organizations/[ORGANIZATION_ID]/logs/[LOG_ID]"
//   - "billingAccounts/[BILLING_ACCOUNT_ID]/logs/[LOG_ID]"
//   - "folders/[FOLDER_ID]/logs/[LOG_ID]"
//
// The function populates the provided resourceAttr map with the extracted ID and log type
// under the appropriate attribute keys. Returns the log type and any error encountered.
func handleLogNameField(logName string, resourceAttr pcommon.Map) (string, error) {
	if logName == "" {
		return "", nil
	}

	addIDsAttributes := func(prefix, format, field string) (string, error) {
		_, rest, _ := strings.Cut(logName, prefix)
		id, logType, _ := strings.Cut(rest, "/logs/")
		if logType == "" || id == "" {
			return "", fmt.Errorf(
				`expected log name %q to have format "%s/%s/logs/[LOG_ID]"`, logName, prefix, format,
			)
		}
		resourceAttr.PutStr(field, id)
		resourceAttr.PutStr(string(conventions.CloudResourceIDKey), logType)
		return logType, nil
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
		return "", fmt.Errorf("unrecognized log name %q", logName)
	}
}

func handlePayload(encodingFormat string, log logEntry, logRecord plog.LogRecord, scope pcommon.InstrumentationScope, cfg Config) error {
	switch encodingFormat {
	case constants.GCPFormatAuditLog:
		// Add encoding.format to scope attributes for audit logs
		scope.Attributes().PutStr(constants.FormatIdentificationTag, encodingFormat)
		if err := auditlog.ParsePayloadIntoAttributes(log.ProtoPayload, logRecord.Attributes()); err != nil {
			return fmt.Errorf("failed to parse audit log proto payload: %w", err)
		}
		return nil
	case constants.GCPFormatVPCFlowLog:
		// Add encoding.format to scope attributes for VPC flow logs
		scope.Attributes().PutStr(constants.FormatIdentificationTag, encodingFormat)
		if err := vpcflowlog.ParsePayloadIntoAttributes(log.JSONPayload, logRecord.Attributes()); err != nil {
			return fmt.Errorf("failed to parse VPC flow log JSON payload: %w", err)
		}
		return nil
	case constants.GCPFormatLoadBalancerLog:
		// Add encoding.format to scope attributes for Load balancer logs
		scope.Attributes().PutStr(constants.FormatIdentificationTag, encodingFormat)
		if err := apploadbalancerlog.ParsePayloadIntoAttributes(log.JSONPayload, logRecord.Attributes()); err != nil {
			return fmt.Errorf("failed to parse Load Balancer log JSON payload: %w", err)
		}
		return nil
		// TODO Add support for more log types
	case constants.GCPFormatProxyNLBLog:
		scope.Attributes().PutStr(constants.FormatIdentificationTag, encodingFormat)
		if err := proxynlb.ParsePayloadIntoAttributes(log.JSONPayload, logRecord.Attributes()); err != nil {
			return fmt.Errorf("failed to parse Proxy NLB log JSON payload: %w", err)
		}
		return nil
	case constants.GCPFormatDNSQueryLog:
		scope.Attributes().PutStr(constants.FormatIdentificationTag, encodingFormat)
		if err := dnslog.ParsePayloadIntoAttributes(log.JSONPayload, logRecord.Attributes()); err != nil {
			return fmt.Errorf("failed to parse DNS Query log JSON payload: %w", err)
		}
		return nil

	case constants.GCPFormatPassthroughNLBLog:
		scope.Attributes().PutStr(constants.FormatIdentificationTag, encodingFormat)
		if err := passthroughnlb.ParsePayloadIntoAttributes(log.JSONPayload, logRecord.Attributes()); err != nil {
			return fmt.Errorf("failed to parse Passthrough NLB log JSON payload: %w", err)
		}
		return nil
		// Fall through to default payload handling for non-armor load balancer logs
		// TODO Add support for more log types
	}

	// if the log type was not recognized, add the payload to the log record body

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

// handleLogEntryFields will place each entry of logEntry as either an attribute of the log,
// or as part of the log body, in case of payload.
func handleLogEntryFields(resourceAttributes pcommon.Map, scopeLogs plog.ScopeLogs, log logEntry, cfg Config) error {
	logRecord := scopeLogs.LogRecords().AppendEmpty()
	scope := scopeLogs.Scope()

	ts := log.Timestamp
	if ts == nil {
		return errors.New("missing timestamp")
	}
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(*ts))

	if receivedTs := log.ReceiveTimestamp; receivedTs != nil {
		logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(*receivedTs))
	}

	shared.PutStr(string(conventions.LogRecordUIDKey), log.InsertID, logRecord.Attributes())

	// Handle log name, get type and encoding format
	logType, errLogName := handleLogNameField(log.LogName, resourceAttributes)
	if errLogName != nil {
		return fmt.Errorf("failed to handle log name field: %w", errLogName)
	}
	encodingFormat := getEncodingFormat(logType)

	if err := handlePayload(encodingFormat, log, logRecord, scope, cfg); err != nil {
		return fmt.Errorf("failed to handle payload field: %w", err)
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
			shared.PutStr(strcase.ToSnakeWithIgnore(fmt.Sprintf("gcp.label.%s", k), "."), v, resourceAttributes)
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

	return nil
}
