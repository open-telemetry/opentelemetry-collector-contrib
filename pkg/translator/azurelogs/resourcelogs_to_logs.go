// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	gojson "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
	"github.com/relvacode/iso8601"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventionsv128 "go.opentelemetry.io/otel/semconv/v1.28.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"
)

const (
	// Constants for OpenTelemetry Specs
	scopeName = "otelcol/azureresourcelogs"

	attributeAzureCategory          = "azure.category"
	attributeAzureCorrelationID     = "azure.correlation_id"
	attributeAzureOperationName     = "azure.operation.name"
	attributeAzureOperationVersion  = "azure.operation.version"
	attributeAzureResultType        = "azure.result.type"
	attributeAzureResultSignature   = "azure.result.signature"
	attributeAzureResultDescription = "azure.result.description"
	attributeEventOriginal          = "event.original"

	// Constants for Azure Log Record body fields
	azureCategory          = "category"
	azureCorrelationID     = "correlation.id"
	azureDuration          = "duration"
	azureIdentity          = "identity"
	azureOperationName     = "operation.name"
	azureOperationVersion  = "operation.version"
	azureProperties        = "properties"
	azureResultType        = "result.type"
	azureResultSignature   = "result.signature"
	azureResultDescription = "result.description"
	azureTenantID          = "tenant.id"

	// Identity claims
	identityClaimEmail = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
)

var errMissingTimestamp = errors.New("missing timestamp")

// as exported via an Azure Event Hub
type azureRecords struct {
	Records []azureLogRecord `json:"records"`
}

// identity describes the identity of the user or application that performed the operation
// described by the log event.
type identity struct {
	// Claims usually contains the JWT token used by Active Directory
	// to authenticate the user or application to perform this
	// operation in Resource Manager.
	Claims map[string]string `json:"claims"`
}

// azureLogRecord represents a single Azure log following
// the common schema:
// https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/resource-logs-schema
type azureLogRecord struct {
	Time              string          `json:"time"`
	Timestamp         string          `json:"timeStamp"`
	ResourceID        string          `json:"resourceId"`
	TenantID          *string         `json:"tenantId"`
	OperationName     string          `json:"operationName"`
	OperationVersion  *string         `json:"operationVersion"`
	Category          string          `json:"category"`
	ResultType        *string         `json:"resultType"`
	ResultSignature   *string         `json:"resultSignature"`
	ResultDescription *string         `json:"resultDescription"`
	DurationMs        *json.Number    `json:"durationMs"`
	CallerIPAddress   *string         `json:"callerIpAddress"`
	CorrelationID     *string         `json:"correlationId"`
	Identity          json.RawMessage `json:"identity"`
	Level             *json.Number    `json:"Level"`
	Location          *string         `json:"location"`
	Properties        json.RawMessage `json:"properties"`
}

var _ plog.Unmarshaler = (*ResourceLogsUnmarshaler)(nil)

type ResourceLogsUnmarshaler struct {
	Version     string
	Logger      *zap.Logger
	TimeFormats []string
}

func (r ResourceLogsUnmarshaler) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	iter := jsoniter.ConfigFastest.BorrowIterator(buf)
	defer jsoniter.ConfigFastest.ReturnIterator(iter)

	var azureLogs azureRecords
	iter.ReadVal(&azureLogs)

	if iter.Error != nil {
		return plog.Logs{}, fmt.Errorf("JSON parse failed: %w", iter.Error)
	}

	var rawRecordMap map[int]json.RawMessage
	getRawRecord := func(index int) json.RawMessage {
		if rawRecordMap == nil {
			// Create rawRecordMap on first use
			var rawRecords struct {
				Records []json.RawMessage `json:"records"`
			}
			if err := json.Unmarshal(buf, &rawRecords); err == nil {
				rawRecordMap = make(map[int]json.RawMessage)
				for i := range rawRecords.Records {
					rawRecordMap[i] = rawRecords.Records[i]
				}
			}
		}
		if rawRecordMap != nil {
			return rawRecordMap[index]
		}
		return nil
	}

	observedTimestamp := pcommon.NewTimestampFromTime(time.Now())

	allResourceScopeLogs := map[string]plog.ScopeLogs{}
	for i := range azureLogs.Records {
		log := &azureLogs.Records[i]
		scopeLogs, found := allResourceScopeLogs[log.ResourceID]
		if !found {
			scopeLogs = plog.NewScopeLogs()
			scopeLogs.Scope().SetName(scopeName)
			scopeLogs.Scope().SetVersion(r.Version)
			allResourceScopeLogs[log.ResourceID] = scopeLogs
		}

		nanos, err := getTimestamp(log, r.TimeFormats...)
		if err != nil {
			r.Logger.Warn("Unable to convert timestamp from log", zap.String("timestamp", log.Time))
			continue
		}

		lr := scopeLogs.LogRecords().AppendEmpty()
		lr.SetTimestamp(nanos)
		lr.SetObservedTimestamp(observedTimestamp)

		if log.Level != nil {
			severity := asSeverity(*log.Level)
			lr.SetSeverityNumber(severity)
			lr.SetSeverityText(log.Level.String())
		}

		err = addRecordAttributes(log.Category, log.Properties, lr)
		if err != nil {
			if errors.Is(err, errStillToImplement) || errors.Is(err, errUnsupportedCategory) {
				// TODO @constanca-m This will be removed once the categories
				// are properly mapped to the semantic conventions in
				// category_logs.go
				rawRecord := getRawRecord(i)
				err = lr.Body().FromRaw(extractRawAttributes(log, rawRecord))
				if err != nil {
					return plog.Logs{}, err
				}
				continue
			}

			correlationID := "unknown"
			if log.CorrelationID != nil {
				correlationID = *log.CorrelationID
			}
			r.Logger.Error(
				"unable to convert log record",
				zap.String("category", log.Category),
				zap.String("resource id", log.ResourceID),
				zap.String("correlation id", correlationID),
				zap.Error(err),
			)
		} else {
			addCommonSchema(log, lr)
		}
	}

	l := plog.NewLogs()
	for resourceID, scopeLogs := range allResourceScopeLogs {
		rl := l.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())
		rl.Resource().Attributes().PutStr(string(conventions.CloudResourceIDKey), resourceID)
		rl.Resource().Attributes().PutStr(string(conventionsv128.EventNameKey), "az.resource.log")
		scopeLogs.MoveTo(rl.ScopeLogs().AppendEmpty())
	}

	return l, nil
}

func getTimestamp(record *azureLogRecord, formats ...string) (pcommon.Timestamp, error) {
	if record.Time != "" {
		return asTimestamp(record.Time, formats...)
	} else if record.Timestamp != "" {
		return asTimestamp(record.Timestamp, formats...)
	}

	return 0, errMissingTimestamp
}

// asTimestamp will parse an ISO8601 string into an OpenTelemetry
// nanosecond timestamp. If the string cannot be parsed, it will
// return zero and the error.
func asTimestamp(s string, formats ...string) (pcommon.Timestamp, error) {
	var err error
	var t time.Time

	// Try parsing with provided formats first
	for _, format := range formats {
		if t, err = time.Parse(format, s); err == nil {
			return pcommon.Timestamp(t.UnixNano()), nil
		}
	}

	// Fallback to ISO 8601 parsing if no format matches
	if t, err = iso8601.ParseString(s); err == nil {
		return pcommon.Timestamp(t.UnixNano()), nil
	}
	return 0, err
}

// asSeverity converts the Azure log level to equivalent
// OpenTelemetry severity numbers. If the log level is not
// valid, then the 'Unspecified' value is returned.
func asSeverity(number json.Number) plog.SeverityNumber {
	switch number.String() {
	case "Informational":
		return plog.SeverityNumberInfo
	case "Warning":
		return plog.SeverityNumberWarn
	case "Error":
		return plog.SeverityNumberError
	case "Critical":
		return plog.SeverityNumberFatal
	default:
		levelNumber, _ := number.Int64()
		if levelNumber > 0 {
			return plog.SeverityNumber(levelNumber)
		}

		return plog.SeverityNumberUnspecified
	}
}

func putStrPtr(field string, value *string, record plog.LogRecord) {
	if value != nil && *value != "" {
		record.Attributes().PutStr(field, *value)
	}
}

func addCommonSchema(log *azureLogRecord, record plog.LogRecord) {
	record.Attributes().PutStr(attributeAzureCategory, log.Category)
	putStrPtr(attributeAzureCorrelationID, log.CorrelationID, record)
	record.Attributes().PutStr(attributeAzureOperationName, log.OperationName)
	putStrPtr(attributeAzureOperationVersion, log.OperationVersion, record)
	putStrPtr(string(conventions.CloudAccountIDKey), log.TenantID, record)
	putStrPtr(attributeAzureResultType, log.ResultType, record)
	putStrPtr(attributeAzureResultSignature, log.ResultSignature, record)
	putStrPtr(attributeAzureResultDescription, log.ResultDescription, record)
	putStrPtr(string(conventions.NetworkPeerAddressKey), log.CallerIPAddress, record)

	addIdentityAttributes(log.Identity, record)
}

func extractRawAttributes(log *azureLogRecord, rawRecord json.RawMessage) map[string]any {
	attrs := map[string]any{}

	attrs[azureCategory] = log.Category
	setIf(attrs, azureCorrelationID, log.CorrelationID)
	if log.DurationMs != nil {
		duration, err := strconv.ParseInt(log.DurationMs.String(), 10, 64)
		if err == nil {
			attrs[azureDuration] = duration
		}
	}
	if len(log.Identity) > 0 {
		var identity any
		if err := gojson.Unmarshal(log.Identity, &identity); err == nil {
			attrs[azureIdentity] = identity
		}
	}
	attrs[azureOperationName] = log.OperationName
	setIf(attrs, azureOperationVersion, log.OperationVersion)

	if log.Properties != nil {
		copyPropertiesAndApplySemanticConventions(log.Category, log.Properties, attrs)
	}

	// The original log needs to be preserved for logs that don't have a properties field
	if len(log.Properties) == 0 && len(rawRecord) > 0 {
		// Format the JSON with proper indentation to match expected output
		var formattedJSON bytes.Buffer
		if err := json.Indent(&formattedJSON, rawRecord, "", "\t"); err == nil {
			attrs[attributeEventOriginal] = formattedJSON.String()
		} else {
			attrs[attributeEventOriginal] = string(rawRecord)
		}
	}

	setIf(attrs, azureResultDescription, log.ResultDescription)
	setIf(attrs, azureResultSignature, log.ResultSignature)
	setIf(attrs, azureResultType, log.ResultType)
	setIf(attrs, azureTenantID, log.TenantID)

	setIf(attrs, string(conventions.CloudRegionKey), log.Location)
	setIf(attrs, string(conventions.NetworkPeerAddressKey), log.CallerIPAddress)
	return attrs
}

func copyPropertiesAndApplySemanticConventions(category string, properties []byte, attrs map[string]any) {
	if len(properties) == 0 {
		return
	}

	// TODO @constanca-m: This is a temporary workaround to
	// this function. This will be removed once category_logs.log
	// is implemented for all currently supported categories
	var props map[string]any
	if err := gojson.Unmarshal(properties, &props); err != nil {
		var val any
		if err = json.Unmarshal(properties, &val); err == nil {
			// Try primitive value
			attrs[azureProperties] = val
		}
		// Parsing failed completely - just return without setting properties
		return
	}

	var handleFunc func(string, any, map[string]any, map[string]any)
	switch category {
	case categoryFrontDoorAccessLog:
		handleFunc = handleFrontDoorAccessLog
	case categoryFrontDoorHealthProbeLog:
		handleFunc = handleFrontDoorHealthProbeLog
	case categoryAppServiceAppLogs:
		handleFunc = handleAppServiceAppLogs
	case categoryAppServiceAuditLogs:
		handleFunc = handleAppServiceAuditLogs
	case categoryAppServiceAuthenticationLogs:
		handleFunc = handleAppServiceAuthenticationLogs
	case categoryAppServiceConsoleLogs:
		handleFunc = handleAppServiceConsoleLogs
	case categoryAppServiceHTTPLogs:
		handleFunc = handleAppServiceHTTPLogs
	case categoryAppServiceIPSecAuditLogs:
		handleFunc = handleAppServiceIPSecAuditLogs
	case categoryAppServicePlatformLogs:
		handleFunc = handleAppServicePlatformLogs
	default:
		handleFunc = func(field string, value any, _, attrsProps map[string]any) {
			attrsProps[field] = value
		}
	}

	attrsProps := map[string]any{}
	for field, value := range props {
		handleFunc(field, value, attrs, attrsProps)
	}
	if len(attrsProps) > 0 {
		attrs[azureProperties] = attrsProps
	}
}

func setIf(attrs map[string]any, key string, value *string) {
	if value != nil && *value != "" {
		attrs[key] = *value
	}
}

// addIdentityAttributes extracts identity details
//
// The `identity` field is part of the Top-level common schema for
// resource logs and it's also in use in the activity logs.
//
// We're applying the strategy to only pick the identity elements
// that we know are useful. This approach also minimizes the risk
// of accidentally including sensitive data.
func addIdentityAttributes(identityJSON json.RawMessage, record plog.LogRecord) {
	var id identity
	if err := gojson.Unmarshal(identityJSON, &id); err != nil {
		return
	}

	// Extract known claims details we want to include in the
	// log record.
	// Extract common claim fields
	if email := id.Claims[identityClaimEmail]; email != "" {
		record.Attributes().PutStr(string(conventions.UserEmailKey), email)
	}
}
