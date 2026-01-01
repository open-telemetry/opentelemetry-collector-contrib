// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"encoding/json"
	"errors"
	"fmt"

	gojson "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

// List of supported Azure Resource Log Categories
const (
	categoryAppServiceAppLogs                  = "AppServiceAppLogs"
	categoryAppServiceAuditLogs                = "AppServiceAuditLogs"
	categoryAppServiceAuthenticationLogs       = "AppServiceAuthenticationLogs"
	categoryAppServiceConsoleLogs              = "AppServiceConsoleLogs"
	categoryAppServiceFileAuditLogs            = "AppServiceFileAuditLogs"
	categoryAppServiceHTTPLogs                 = "AppServiceHTTPLogs"
	categoryAppServiceIPSecAuditLogs           = "AppServiceIPSecAuditLogs"
	categoryAppServicePlatformLogs             = "AppServicePlatformLogs"
	categoryAzureCdnAccessLog                  = "AzureCdnAccessLog"
	categoryFrontDoorAccessLog                 = "FrontDoorAccessLog"
	categoryFrontDoorHealthProbeLog            = "FrontDoorHealthProbeLog"
	categoryFrontdoorWebApplicationFirewallLog = "FrontDoorWebApplicationFirewallLog"
	categoryRecommendation                     = "Recommendation"
)

// Non-SemConv attributes that are used for common Azure Log Record fields
const (
	// OpenTelemetry attribute name for Azure Correlation ID,
	// from `correlationId` field in Azure Log Record
	attributeAzureCorrelationID = "azure.correlation_id"

	// OpenTelemetry attribute name for Azure Operation Version,
	// from `operationVersion` field in Azure Log Record
	attributeAzureOperationVersion = "azure.operation.version"

	// OpenTelemetry attribute name for generic Azure Operation Duration,
	// from `durationMs` field in Azure Log Record
	attributeAzureOperationDuration = "azure.operation.duration"

	// OpenTelemetry attribute name for Azure Identity,
	// from `identity` field in Azure Log Record
	attributeAzureIdentity = "azure.identity"

	// OpenTelemetry attribute name for Azure Log Record properties,
	// from `properties` field in Azure Log Record
	// Used when we cannot map parse "properties" field or
	// cannot map parsed "properties" to attributes directly
	attributesAzureProperties = "azure.properties"

	// OpenTelemetry attribute name for Azure Result Type,
	// from `resultType` field in Azure Log Record
	attributeAzureResultType = "azure.result.type"

	// OpenTelemetry attribute name for Azure Result Signature,
	// from `resultSignature` field in Azure Log Record
	attributeAzureResultSignature = "azure.result.signature"

	// OpenTelemetry attribute name for Azure Result Description,
	// from `resultDescription` field in Azure Log Record
	attributesAzureResultDescription = "azure.result.description"
)

// Common Non-SemConv attributes that are used in "properties" fields across multiple
// Azure Log Record Categories
const (
	// OpenTelemetry attribute name for "Host" HTTP Header value
	attributeHTTPHeaderHost = "http.request.header.host"

	// OpenTelemetry attribute name for "Referer" HTTP Header value
	attributeHTTPHeaderReferer = "http.request.header.referer"

	// OpenTelemetry attribute name for Azure HTTP Request Duration,
	// from `durationMs` field in Azure Log Record
	attributeAzureRequestDuration = "azure.request.duration"
)

var errNoTimestamp = errors.New("no valid time fields are set on Log record ('time' or 'timestamp')")

// azureLogRecord is a common interface for all category-specific structures
type azureLogRecord interface {
	GetResource() logsResourceAttributes
	GetTimestamp(formats ...string) (pcommon.Timestamp, error)
	GetLevel() (plog.SeverityNumber, string, bool)
	PutCommonAttributes(attrs pcommon.Map, body pcommon.Value)
	PutProperties(attrs pcommon.Map, body pcommon.Value) error
}

// azureLogRecordBase represents a single Azure log following the common schema:
// https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/resource-logs-schema
// This schema are applicable to most Resource Logs and
// can be extended with additional fields for specific Log Categories
type azureLogRecordBase struct {
	Time              string          `json:"time"`      // most Categories use this field for timestamp
	TimeStamp         string          `json:"timestamp"` // some Categories use this field for timestamp
	ResourceID        string          `json:"resourceId"`
	TenantID          string          `json:"tenantId"`
	OperationName     string          `json:"operationName"`
	OperationVersion  *string         `json:"operationVersion"`
	ResultType        *string         `json:"resultType"`
	ResultSignature   *string         `json:"resultSignature"`
	ResultDescription *string         `json:"resultDescription"`
	DurationMs        *json.Number    `json:"durationMs"` // int
	CallerIPAddress   *string         `json:"callerIpAddress"`
	CorrelationID     *string         `json:"correlationId"`
	Identity          *map[string]any `json:"identity"`
	Level             *string         `json:"level"`
	Location          string          `json:"location"`
}

// GetResource returns resource attributes for the parsed Log Record
// As for now it includes ResourceID, TenantID and Location
func (r *azureLogRecordBase) GetResource() logsResourceAttributes {
	return logsResourceAttributes{
		ResourceID: r.ResourceID,
		TenantID:   r.TenantID,
		Location:   r.Location,
	}
}

// GetTimestamp tries to parse timestamp from either `time` or `timestamp` fields
// using provided list of time formats.
// If both fields are empty (undefined), or parsing failed - return an error
func (r *azureLogRecordBase) GetTimestamp(formats ...string) (pcommon.Timestamp, error) {
	if r.Time == "" && r.TimeStamp == "" {
		return pcommon.Timestamp(0), errNoTimestamp
	}

	time := r.Time
	if time == "" {
		time = r.TimeStamp
	}

	nanos, err := unmarshaler.AsTimestamp(time, formats...)
	if err != nil {
		return pcommon.Timestamp(0), fmt.Errorf("unable to convert value %q as timestamp: %w", time, err)
	}

	return nanos, nil
}

// GetLevel tries to convert the Log Level into OpenTelemetry SeverityNumber
// If level is not set - return SeverityNumberUnspecified and flag that level is not set
// If level is set, but invalid - return SeverityNumberUnspecified and flag that level is set
func (r *azureLogRecordBase) GetLevel() (plog.SeverityNumber, string, bool) {
	if r.Level == nil {
		return plog.SeverityNumberUnspecified, "", false
	}

	severity := asSeverity(*r.Level)
	// Saving original log.Level text,
	// not the internal OpenTelemetry SeverityNumber -> SeverityText mapping
	return severity, *r.Level, true
}

// PutCommonAttributes puts already parsed common attributes into provided Attributes Map/Body
func (r *azureLogRecordBase) PutCommonAttributes(attrs pcommon.Map, _ pcommon.Value) {
	// Common fields for all Azure Resource Log Categories should be
	// placed as attributes, no matter if we can map the category or not
	unmarshaler.AttrPutStrIf(attrs, unmarshaler.AttributeAzureOperationName, r.OperationName)
	unmarshaler.AttrPutStrPtrIf(attrs, attributeAzureOperationVersion, r.OperationVersion)
	unmarshaler.AttrPutStrPtrIf(attrs, attributeAzureResultType, r.ResultType)
	unmarshaler.AttrPutStrPtrIf(attrs, attributeAzureResultSignature, r.ResultSignature)
	unmarshaler.AttrPutStrPtrIf(attrs, attributesAzureResultDescription, r.ResultDescription)
	unmarshaler.AttrPutStrPtrIf(attrs, string(conventions.NetworkPeerAddressKey), r.CallerIPAddress)
	unmarshaler.AttrPutStrPtrIf(attrs, attributeAzureCorrelationID, r.CorrelationID)
	unmarshaler.AttrPutIntNumberPtrIf(attrs, attributeAzureOperationDuration, r.DurationMs)
	if r.Identity != nil {
		unmarshaler.AttrPutMapIf(attrs, attributeAzureIdentity, *r.Identity)
	}
}

// PutProperties puts already attributes from "properties" field into provided Attributes Map/Body
// MUST be implemented by each specific logCategory structure if "properties" field is expected there
func (*azureLogRecordBase) PutProperties(_ pcommon.Map, _ pcommon.Value) error {
	// By default - no "properties", so nothing to do here
	return nil
}

// azureLogRecordBase represents a single Azure log following the common schema,
// but has unknown for us Category
// In this case we couldn't correctly map properties to attributes and simply copy them
// as-is to the attributes
type azureLogRecordGeneric struct {
	azureLogRecordBase

	Properties json.RawMessage `json:"properties"`
}

func (r *azureLogRecordGeneric) PutProperties(attrs pcommon.Map, body pcommon.Value) error {
	var properties map[string]any

	if len(r.Properties) == 0 {
		// Nothing to parse
		return nil
	}

	// We expect "properties" to be a correct JSON object in most cases,
	// so we'll try to parse it as JSON here
	// If parsing will fail - we will put value of "properties" field
	// into `azure.properties` Attribute and return parse error to caller
	if err := gojson.Unmarshal(r.Properties, &properties); err != nil {
		attrs.PutStr(attributesAzureProperties, string(r.Properties))
		return fmt.Errorf("failed to parse Azure Logs 'properties' field as JSON: %w", err)
	}

	// Put everything into attributes
	for k, v := range properties {
		switch k {
		case "Message", "message":
			if err := body.FromRaw(v); err != nil {
				body.SetStr(fmt.Sprintf("%v", v))
			}
		case "correlationId":
			value := attrs.PutEmpty(attributeAzureCorrelationID)
			if err := value.FromRaw(v); err != nil {
				value.SetStr(fmt.Sprintf("%v", v))
			}
		case "duration":
			value := attrs.PutEmpty(attributeAzureOperationDuration)
			if err := value.FromRaw(v); err != nil {
				value.SetStr(fmt.Sprintf("%v", v))
			}
		default:
			// Keep all other fields as-is
			value := attrs.PutEmpty(k)
			if err := value.FromRaw(v); err != nil {
				value.SetStr(fmt.Sprintf("%v", v))
			}
		}
	}

	return nil
}

// processLogRecord tries to parse incoming record based of provided logCategory
func processLogRecord(logCategory string, record []byte) (azureLogRecord, error) {
	var parsed azureLogRecord

	switch logCategory {
	case categoryAppServiceAppLogs:
		parsed = new(azureAppServiceAppLog)
	case categoryAppServiceAuditLogs:
		parsed = new(azureAppServiceAuditLog)
	case categoryAppServiceAuthenticationLogs:
		parsed = new(azureAppServiceAuthenticationLog)
	case categoryAppServiceConsoleLogs:
		parsed = new(azureAppServiceConsoleLog)
	case categoryAppServiceHTTPLogs:
		parsed = new(azureAppServiceHTTPLog)
	case categoryAppServiceIPSecAuditLogs:
		parsed = new(azureAppServiceIPSecAuditLog)
	case categoryAppServicePlatformLogs:
		parsed = new(azureAppServicePlatformLog)
	case categoryAppServiceFileAuditLogs:
		parsed = new(azureAppServiceFileAuditLog)
	case categoryAzureCdnAccessLog:
		parsed = new(azureHTTPAccessLog)
	case categoryFrontDoorAccessLog:
		parsed = new(azureHTTPAccessLog)
	case categoryFrontDoorHealthProbeLog:
		parsed = new(frontDoorHealthProbeLog)
	case categoryFrontdoorWebApplicationFirewallLog:
		parsed = new(frontDoorWAFLog)
	case categoryRecommendation:
		parsed = new(azureRecommendationLog)
	default:
		parsed = new(azureLogRecordGeneric)
	}

	// Unfortunately, "goccy/go-json" has a bug with case-insensitive key matching
	// for nested structures, so we have to use jsoniter here
	// see https://github.com/goccy/go-json/issues/470
	if err := jsoniter.ConfigFastest.Unmarshal(record, parsed); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w", err)
	}

	return parsed, nil
}
