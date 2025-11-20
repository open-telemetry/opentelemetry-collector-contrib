// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	gojson "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
	"github.com/relvacode/iso8601"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"
)

const (
	// Constants for OpenTelemetry Specs
	scopeName = "otelcol/azureresourcelogs"

	attributeAzureCategory         = "azure.category"
	attributeAzureCorrelationID    = "azure.correlation_id"
	attributeAzureOperationName    = "azure.operation.name"
	attributeAzureOperationVersion = "azure.operation.version"

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
)

var errMissingTimestamp = errors.New("missing timestamp")

// as exported via an Azure Event Hub
type azureRecords struct {
	Records []azureLogRecord `json:"records"`
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
	Identity          *any            `json:"identity"`
	Level             *json.Number    `json:"Level"`
	Location          *string         `json:"location"`
	Properties        json.RawMessage `json:"properties"`
	// rawRecord stores the raw JSON bytes of the entire record to capture
	// fields that aren't in the struct (e.g., vnet flow log fields)
	rawRecord json.RawMessage
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

	// Re-parse to capture raw record JSON for fields not in the struct
	var rawRecords struct {
		Records []json.RawMessage `json:"records"`
	}
	if err := json.Unmarshal(buf, &rawRecords); err == nil {
		for i := range azureLogs.Records {
			if i < len(rawRecords.Records) {
				azureLogs.Records[i].rawRecord = rawRecords.Records[i]
			}
		}
	}

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
				err = lr.Body().FromRaw(extractRawAttributes(log))
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
		rl.Resource().Attributes().PutStr(string(conventions.EventNameKey), "az.resource.log")
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
	// TODO Keep adding other common fields, like tenant ID
}

// parseRawRecord splits known vs unknown fields from the raw record JSON.
func parseRawRecord(log *azureLogRecord) (map[string]any, map[string]any) {
	if len(log.rawRecord) == 0 {
		return nil, nil
	}

	var raw map[string]any
	if err := gojson.Unmarshal(log.rawRecord, &raw); err != nil {
		return nil, nil
	}

	// Extract all fields from raw record, excluding known struct fields
	// Use case-insensitive matching to handle both camelCase and PascalCase JSON formats
	// Map uses lowercase keys for case-insensitive O(1) lookup
	knownFields := map[string]bool{
		"time":              true,
		"timestamp":         true,
		"resourceid":        true,
		"tenantid":          true,
		"operationname":     true,
		"operationversion":  true,
		"category":          true,
		"resulttype":        true,
		"resultsignature":   true,
		"resultdescription": true,
		"durationms":        true,
		"calleripaddress":   true,
		"correlationid":     true,
		"identity":          true,
		"level":             true,
		"location":          true,
		"properties":        true,
	}

	known := make(map[string]any)
	unknown := make(map[string]any)

	for k, v := range raw {
		if knownFields[strings.ToLower(k)] {
			known[k] = v
		} else {
			unknown[k] = v
		}
	}
	return known, unknown
}

// addCommonAzureFields fills attrs with the standard Azure fields.
func addCommonAzureFields(attrs map[string]any, log *azureLogRecord) {
	attrs[azureCategory] = log.Category
	setIf(attrs, azureCorrelationID, log.CorrelationID)
	if log.DurationMs != nil {
		duration, err := strconv.ParseInt(log.DurationMs.String(), 10, 64)
		if err == nil {
			attrs[azureDuration] = duration
		}
	}
	if log.Identity != nil {
		attrs[azureIdentity] = *log.Identity
	}
	attrs[azureOperationName] = log.OperationName
	setIf(attrs, azureOperationVersion, log.OperationVersion)
	setIf(attrs, azureResultDescription, log.ResultDescription)
	setIf(attrs, azureResultSignature, log.ResultSignature)
	setIf(attrs, azureResultType, log.ResultType)
	setIf(attrs, azureTenantID, log.TenantID)

	setIf(attrs, string(conventions.CloudRegionKey), log.Location)
	setIf(attrs, string(conventions.NetworkPeerAddressKey), log.CallerIPAddress)
	return attrs
}

func extractRawAttributes(log *azureLogRecord) map[string]any {
	attrs := map[string]any{}
	_, unknown := parseRawRecord(log)

	// TODO @constanca-m: This is a temporary workaround to
	// this function. This will be removed once category_logs.log
	// is implemented for all currently supported categories
	var props map[string]any
	var propsVal any

	// Try to parse properties as a map first
	if len(log.Properties) > 0 {
		if err := gojson.Unmarshal(log.Properties, &props); err != nil {
			// If not a map, try to parse as a primitive value
			if err := json.Unmarshal(log.Properties, &propsVal); err != nil {
				// If parsing fails, treat as string
				propsVal = string(log.Properties)
			}
		}
	}

	// Merge unknown fields into props
	if len(unknown) > 0 {
		if props == nil {
			props = make(map[string]any)
		}
		// If we have a primitive property, add it to the map under 'properties' key
		// so it is preserved when merging with unknown fields
		if propsVal != nil {
			props[azureProperties] = propsVal
			propsVal = nil
		}
		for k, v := range unknown {
			props[k] = v
		}
	}

	// Apply semantic conventions if we have a map
	if props != nil {
		remainingProps := applySemanticConventions(log.Category, props, attrs)
		if len(remainingProps) > 0 {
			attrs[azureProperties] = remainingProps
		}
	} else if propsVal != nil {
		attrs[azureProperties] = propsVal
	}

	// Add standardized Azure fields
	addCommonAzureFields(attrs, log)
	return attrs
}

// applySemanticConventions extracts known fields into attrs and returns the remaining fields.
func applySemanticConventions(category string, props, attrs map[string]any) map[string]any {
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

	remainingProps := make(map[string]any)
	for field, value := range props {
		handleFunc(field, value, attrs, remainingProps)
	}
	return remainingProps
}

func setIf(attrs map[string]any, key string, value *string) {
	if value != nil && *value != "" {
		attrs[key] = *value
	}
}
