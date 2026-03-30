// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"encoding/json"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

const (
	// OpenTelemetry attribute name for Event ID
	attributeAzureEventID = "azure.event.id"

	// OpenTelemetry attribute name for Event Name
	attributeAzureEventName = "azure.event.name"
)

// Sometimes Azure sends invalid JSON as a string with a single-quoted keys/values
// for example: `"properties": "{'key':'value'}"`
// instead of `"properties": {"key":"value"}`
// So here a specific wrapper to make this JSON valid
type azureFunctionAppLogProperties struct {
	ActivityID           string      `json:"activityId"`
	AppName              string      `json:"appName"`
	Category             string      `json:"category"`
	EventID              json.Number `json:"eventId"` // int
	EventName            string      `json:"eventName"`
	ExceptionDetails     string      `json:"exceptionDetails"`
	ExceptionMessage     string      `json:"exceptionMessage"`
	ExceptionType        string      `json:"exceptionType"`
	FunctionInvocationID string      `json:"functionInvocationId"`
	FunctionName         string      `json:"functionName"`
	HostInstanceID       string      `json:"hostInstanceId"`
	HostVersion          string      `json:"hostVersion"`
	Level                string      `json:"level"`
	LevelID              json.Number `json:"levelId"` // int
	Message              string      `json:"message"`
	ProcessID            json.Number `json:"processId"` // int
	RoleInstanceID       string      `json:"roleInstance"`
}

func (p *azureFunctionAppLogProperties) UnmarshalJSON(data []byte) error {
	if len(data) > 1 && data[0] == '"' && data[1] == '{' {
		// Remove leading and trailing double quote
		data = convertInvalidSingleQuotedJSON(data[1 : len(data)-1])
	}

	// Define an alias type to avoid infinite recursion
	type alias azureFunctionAppLogProperties
	var temp alias

	if err := jsoniter.ConfigFastest.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Assign the unmarshaled fields from the alias to the original struct
	*p = azureFunctionAppLogProperties(temp)

	return nil
}

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/functionapplogs
// There is no documentation about the structure of the logs, so we will
// implement this based on the fields from Azure Monitor table excluding generic fields
type azureFunctionAppLog struct {
	azureLogRecordBase

	Properties       json.RawMessage                `json:"properties"`
	parsedProperties *azureFunctionAppLogProperties `json:"-"`
	propertiesParsed bool                           `json:"-"`
}

// Override GetResource to add ServiceName and ServiceInstanceID from Properties
func (r *azureFunctionAppLog) GetResource() logsResourceAttributes {
	// Try to parse Properties to get AppName and FunctionName
	properties := r.getParsedProperties()
	if properties == nil {
		// We failed to parse "properties" - return basic Resource attributes only
		return logsResourceAttributes{
			ResourceID: r.ResourceID,
			TenantID:   r.TenantID,
			Location:   r.Location,
		}
	}

	// In general "appName" is naturally matched to "service.name" Resource Attribute,
	// but in Azure multiple functions could be deployed into single FunctionApp
	// So to make "service.name" correctly identifiable we will use the same approach
	// as in SemConv "faas.name" - combine "appName" and "functionName"
	serviceName := properties.AppName
	if properties.FunctionName != "" {
		serviceName = properties.AppName + "/" + properties.FunctionName
	}
	return logsResourceAttributes{
		ResourceID:  r.ResourceID,
		TenantID:    r.TenantID,
		Location:    r.Location,
		ServiceName: serviceName,
		// "RoleInstance" field typically matches "AppRoleInstance" in Azure Traces, so we'll assign it to the
		// same attribute as "service.instance.id" as in Trace Unmarshaler for correlation purposes
		ServiceInstanceID: properties.RoleInstanceID,
	}
}

func (r *azureFunctionAppLog) PutProperties(attrs pcommon.Map, body pcommon.Value) error {
	// Put some common attributes
	unmarshaler.AttrPutStrIf(attrs, string(conventions.FaaSInvokedProviderKey), conventions.CloudProviderAzure.Value.AsString())

	// Try to parse Properties field
	properties := r.getParsedProperties()
	if properties == nil {
		// Properties field could not be parsed - put raw string to `azure.properties` attribute
		attrs.PutStr(attributesAzureProperties, string(r.Properties))
		return nil
	}

	// If we were able to parse Properties - put all known fields to attributes
	unmarshaler.AttrPutStrIf(attrs, string(conventions.LogRecordUIDKey), properties.ActivityID)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureEventName, properties.EventName)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeAzureEventID, properties.EventID)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.FaaSInvocationIDKey), properties.FunctionInvocationID)
	// According to SemConv this attribute for Azure should be in form `<FUNCAPP>/<FUNC>`
	// For clear func name we will use faas.invoked_name attribute
	unmarshaler.AttrPutStrIf(attrs, string(conventions.FaaSNameKey), properties.AppName+"/"+properties.FunctionName)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.FaaSInvokedNameKey), properties.FunctionName)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.HostIDKey), properties.HostInstanceID)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.HostImageVersionKey), properties.HostVersion)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.ProcessPIDKey), properties.ProcessID)
	// "exceptionDetails" field typically contains a full stack trace of the exception
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ExceptionStacktraceKey), properties.ExceptionDetails)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ExceptionMessageKey), properties.ExceptionMessage)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ExceptionTypeKey), properties.ExceptionType)

	body.SetStr(properties.Message)

	return nil
}

func (r *azureFunctionAppLog) getParsedProperties() *azureFunctionAppLogProperties {
	if r.propertiesParsed {
		return r.parsedProperties
	}

	var properties azureFunctionAppLogProperties
	r.propertiesParsed = true
	if err := jsoniter.ConfigFastest.Unmarshal(r.Properties, &properties); err != nil {
		return nil
	}
	r.parsedProperties = &properties

	return r.parsedProperties
}
