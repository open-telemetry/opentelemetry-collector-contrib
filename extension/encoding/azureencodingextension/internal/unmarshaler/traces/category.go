// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/traces"

import (
	"errors"
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

// List of supported "Logs" Categories (in Azure - Traces are actually Logs inside)
// This Categories has minimal set of required fields to be transformed into Traces:
// Timestamp ("time"), TraceId ("OperationId") and SpanId ("Id")
// Other Categories in `microsoft.insights/components` Resource type doesn't have some fields,
// like Id (SpanId) or looks more like Logs or Metrics:
// - AppBrowserTimings - seems to be Metrics
// - AppEvents - seems to be Logs
// - AppExceptions - seems to be Logs
// - AppMetrics - seems to be Metrics
// - AppPerformanceCounters - seems to be Metrics
// - AppSystemEvents - seems to be Logs
// - AppTraces -seems to be Logs
// This Categories is missing sample data to properly test them:
// - AppPageViews
const (
	categoryAvailabilityResults = "AppAvailabilityResults"
	categoryDependencies        = "AppDependencies"
	categoryRequests            = "AppRequests"
)

const (
	// OpenTelemetry attribute name for Azure Client Country or Region,
	// from `ClientCountryOrRegion` field in Azure Trace Record
	attributeGeoCountryName = "geo.country.name"

	// OpenTelemetry attribute name for Azure Client State or Province,
	// from `ClientStateOrProvince` field in Azure Trace Record
	attributeGeoStateName = "geo.state.name"

	// OpenTelemetry attribute name for Azure Client device type,
	// from `ClientType` field in Azure Trace Record
	attributeDeviceType = "device.type"
)

var errNoTimestamp = errors.New("no valid time fields are set on Trace record ('time' field)")

// azureTraceRecord is a common interface for all category-specific structures
type azureTraceRecord interface {
	GetResource() traceResourceAttributes
	GetTimestamp(formats ...string) (pcommon.Timestamp, error)
	GetTraceID() (pcommon.TraceID, error)
	GetSpanID() (pcommon.SpanID, error)
	GetParentSpanID() (pcommon.SpanID, error)
	GetSpanKind() ptrace.SpanKind
	GetSpanName() string
	GetSpanDuration() pcommon.Timestamp
	GetSpanStatus() (ptrace.StatusCode, string)
	PutCommonAttributes(attrs pcommon.Map)
	PutProperties(attrs pcommon.Map)
}

type azureTracesRecordBase struct {
	// Azure Trace Records based on supported tables data
	// the common record schema reference:
	// https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/apprequests
	// https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appdependencies
	// https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appavailabilityresults
	// Common fields
	AppRoleInstance       string            `json:"AppRoleInstance"`       // Role instance of the application
	AppRoleName           string            `json:"AppRoleName"`           // Role name of the application
	AppVersion            string            `json:"AppVersion"`            // Version of the application
	ClientBrowser         string            `json:"ClientBrowser"`         // Browser running on the client device
	ClientCity            string            `json:"ClientCity"`            // City where the client device is located
	ClientCountryOrRegion string            `json:"ClientCountryOrRegion"` // Country or region where the client device is located
	ClientIP              string            `json:"ClientIP"`              // IP address of the client device
	ClientModel           string            `json:"ClientModel"`           // Model of the client device
	ClientOS              string            `json:"ClientOS"`              // Operating system of the client device
	ClientStateOrProvince string            `json:"ClientStateOrProvince"` // State or province where the client device is located
	ClientType            string            `json:"ClientType"`            // Type of the client device
	DurationMs            float64           `json:"DurationMs"`            // Number of milliseconds it took to finish the test
	SpanID                string            `json:"Id"`                    // Unique ID of operation
	Name                  string            `json:"Name"`                  // Operation name, i.e. URI/SQL table name/Availability Test Name/ etc.
	OperationID           string            `json:"OperationId"`           // Application-defined operation ID
	OperationName         string            `json:"OperationName"`         // Application-defined name of the overall operation, typically match the `Name` field
	ParentID              string            `json:"ParentId"`              // ID of the parent operation
	Properties            map[string]string `json:"Properties"`            // Application-defined properties
	ResourceID            string            `json:"resourceId"`            // A unique identifier for the resource that the record is associated with
	SDKVersion            string            `json:"SDKVersion"`            // Version of the SDK used by the application to generate this telemetry item
	SessionID             string            `json:"SessionId"`             // Application-defined session ID
	Success               bool              `json:"Success"`               // Indicates whether the application handled the request successfully
	Time                  string            `json:"time"`                  // Date and time when Operation result was recorded
	Category              string            `json:"Type"`                  // The name of the table (Azure Logs Category)
	UserID                string            `json:"UserId"`                // Anonymous ID of a user accessing the application
}

// GetResource returns resource attributes for the parsed Trace Record
// Implementation is common for all _supported_ Azure Trace Records
func (r *azureTracesRecordBase) GetResource() traceResourceAttributes {
	return traceResourceAttributes{
		ResourceID:      r.ResourceID,
		ServiceName:     r.AppRoleName,
		ServiceInstance: r.AppRoleInstance,
		ServiceVersion:  r.AppVersion,
		SDKVersion:      r.SDKVersion,
	}
}

// GetTimestamp tries to parse timestamp from `time` field using provided list of time formats.
// If field is empty (undefined) or parsing failed - return an error
func (r *azureTracesRecordBase) GetTimestamp(formats ...string) (pcommon.Timestamp, error) {
	if r.Time == "" {
		return pcommon.Timestamp(0), errNoTimestamp
	}

	nanos, err := unmarshaler.AsTimestamp(r.Time, formats...)
	if err != nil {
		return pcommon.Timestamp(0), fmt.Errorf("unable to convert value %q as timestamp: %w", r.Time, err)
	}

	return nanos, nil
}

// GetTraceID tries to parse TraceID from `OperationId` field
// If field is empty (undefined) or parsing failed - return an error
func (r *azureTracesRecordBase) GetTraceID() (pcommon.TraceID, error) {
	traceID, err := trace.TraceIDFromHex(r.OperationID)
	if err != nil {
		return pcommon.TraceID{}, fmt.Errorf("unable to parse TraceID from Azure OperationId %q: %w", r.OperationID, err)
	}
	return pcommon.TraceID(traceID), nil
}

// GetSpanID tries to parse SpanID from `OperationId` and `Id` fields
// If fields are empty (undefined) or parsing failed - return an error
func (r *azureTracesRecordBase) GetSpanID() (pcommon.SpanID, error) {
	// Sometimes Azure sets "Id" equal to "OperationId", i.e. 16-bytes length
	// To avoid loosing of such Spans we will trim Span ID to correct 8-bytes length here
	spanID := r.SpanID
	if r.SpanID == r.OperationID {
		spanID = spanID[:16]
	}

	id, err := trace.SpanIDFromHex(spanID)
	if err != nil {
		return pcommon.SpanID{}, fmt.Errorf("unable to parse SpanID from Azure OperationId/Id %q/%q: %w", r.OperationID, r.SpanID, err)
	}

	return pcommon.SpanID(id), nil
}

// GetParentSpanID tries to parse ParentSpanID from `OperationId` and `ParentId` fields
// If fields are empty (undefined) or parsing failed - return an error
func (r *azureTracesRecordBase) GetParentSpanID() (pcommon.SpanID, error) {
	// Sometimes Azure sets "ParentId" equal to "OperationId", i.e. 16-bytes array
	// We assume that it's actually a Root Span, so set it to empty here
	if r.ParentID == r.OperationID || r.ParentID == "" {
		return pcommon.NewSpanIDEmpty(), nil
	}

	id, err := trace.SpanIDFromHex(r.ParentID)
	if err != nil {
		return pcommon.NewSpanIDEmpty(), fmt.Errorf("unable to parse ParentSpanID from Azure OperationId/ParentId %q/%q: %w", r.OperationID, r.ParentID, err)
	}

	return pcommon.SpanID(id), nil
}

// GetSpanKind is a helper function to determine the SpanKind based on
// the Azure Category and Azure Dependency Type
// MUST be implemented by each specific Trace Record structure,
// because depends on Category-specific fields
// By default - returns "Internal" Span Kind as of OpenTelemetry SemConv Spec
func (*azureTracesRecordBase) GetSpanKind() ptrace.SpanKind {
	return ptrace.SpanKindInternal
}

// GetSpanName returns Span Name from either `Name` or `OperationName` field
func (r *azureTracesRecordBase) GetSpanName() string {
	if r.Name != "" {
		return r.Name
	}

	return r.OperationName
}

// GetSpanDuration returns Span Duration calculated from `DurationMs` field
func (r *azureTracesRecordBase) GetSpanDuration() pcommon.Timestamp {
	return pcommon.Timestamp(r.DurationMs * 1e6) // milliseconds to nanoseconds
}

// GetSpanStatus returns Span Status Code and optional Status Message,
// based on `Success` field
func (r *azureTracesRecordBase) GetSpanStatus() (ptrace.StatusCode, string) {
	// According to Azure Docs if `Success` is false - the operation failed,
	// so we'll mark Span Status as Error for such cases according to OpenTelemetry Specs
	if !r.Success {
		return ptrace.StatusCodeError, ""
	}

	// In all other cases - return Unset Status Code as recommended by OpenTelemetry Specs
	return ptrace.StatusCodeUnset, ""
}

// PutCommonAttributes puts already parsed common attributes into provided Attributes Map/Body
func (r *azureTracesRecordBase) PutCommonAttributes(attrs pcommon.Map) {
	// Common fields for all Azure Trace Categories should be
	// placed as attributes, no matter if we can map the category or not
	unmarshaler.AttrPutStrIf(attrs, string(conventions.UserAgentOriginalKey), r.ClientBrowser)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.GeoLocalityNameKey), r.ClientCity)
	unmarshaler.AttrPutStrIf(attrs, attributeGeoCountryName, r.ClientCountryOrRegion)
	unmarshaler.AttrPutHostPortIf(attrs, string(conventions.ClientAddressKey), string(conventions.ClientPortKey), r.ClientIP)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.DeviceModelNameKey), r.ClientModel)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.UserAgentOSNameKey), r.ClientOS)
	unmarshaler.AttrPutStrIf(attrs, attributeGeoStateName, r.ClientStateOrProvince)
	unmarshaler.AttrPutStrIf(attrs, attributeDeviceType, r.ClientType)
	unmarshaler.AttrPutStrIf(attrs, unmarshaler.AttributeAzureOperationName, r.OperationName)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.SessionIDKey), r.SessionID)
	unmarshaler.AttrPutStrIf(attrs, unmarshaler.AttributeAzureCategory, r.Category)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.UserIDKey), r.UserID)
}

// PutProperties puts already attributes from "properties" field into provided Attributes Map/Body
func (r *azureTracesRecordBase) PutProperties(attrs pcommon.Map) {
	// Properties are free-form attributes so will be copied into Span Attributes as is
	// TODO: Add best-effort parsing of the "properties" to SemConv attributes
	// after we will get more details and/or more samples of data
	for key, value := range r.Properties {
		unmarshaler.AttrPutStrIf(attrs, key, value)
	}
}

// processTraceRecord tries to parse incoming record based of provided traceCategory
func processTraceRecord(traceCategory string, record []byte) (azureTraceRecord, error) {
	var parsed azureTraceRecord

	switch traceCategory {
	case categoryRequests:
		parsed = new(azureAppRequests)
	case categoryDependencies:
		parsed = new(azureAppDependencies)
	case categoryAvailabilityResults:
		parsed = new(azureAppAvailabilityResults)
	default:
		parsed = new(azureTracesRecordBase)
	}

	// Unfortunately, "goccy/go-json" has a bug with case-insensitive key matching
	// for nested structures, so we have to use jsoniter here
	// see https://github.com/goccy/go-json/issues/470
	if err := jsoniter.ConfigFastest.Unmarshal(record, parsed); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w", err)
	}

	return parsed, nil
}
