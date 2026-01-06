// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

// Non-SemConv attributes that are used for common Azure Messaging Log Record fields
const (
	// OpenTelemetry attribute name for Azure Scale Unit name
	attributeAzureMSScaleUnit = "azure.messaging.scale_unit"

	// OpenTelemetry attribute name for Azure Entity name
	// Might be either EventHub name or Queue name as well as other, non-service related entities
	attributeAzureMSEntityName = "azure.messaging.entity.name"

	// OpenTelemetry attribute name for Azure Entity type (e.g. EventHub, Queue, etc.)
	attributeAzureMSEntityType = "azure.messaging.entity.type"
)

// Non-SemConv attributes specific for each Azure Messaging Log Record types
const (
	// OpenTelemetry attribute name for Task Name
	attributeAzureMSTaskName = "azure.messaging.task.name"

	// OpenTelemetry attribute name for Azure Messaging Error Count
	attributeAzureMSErrorCount = "azure.messaging.error.count"

	// OpenTelemetry attribute name for Azure Messaging Entity Child Name
	attributeAzureMSEntityChildName = "azure.messaging.entity.child_name"

	// OpenTelemetry attribute name for Azure Messaging Entity Child Type
	attributeAzureMSEntityChildType = "azure.messaging.entity.child_type"

	// OpenTelemetry attribute name for Partition ID
	attributeAzureMSPartitionID = "azure.messaging.partition_id"

	// OpenTelemetry attribute name for Azure Messaging Auth Type (Microsoft Entra ID or SAS Policy)
	attributeAzureMSAuthType = "azure.messaging.auth.type"

	// OpenTelemetry attribute name for Azure Messaging Auth ID (Microsoft Entra application ID or SAS policy name)
	attributeAzureMSAuthID = "azure.messaging.auth.id"

	// OpenTelemetry attribute name for Azure Messaging Count
	// Total number of operations performed during the aggregated period of 1 minute
	attributeAzureMSCount = "azure.messaging.count"

	// OpenTelemetry attribute name for the status of activity (success or failure)
	attributeAzureMSStatus = "azure.messaging.status"

	// OpenTelemetry attribute name for the caller of operation (the Azure portal or management client)
	attributeAzureMSCaller = "azure.messaging.caller"

	// OpenTelemetry attribute name for the reason why the action was done
	attributeAzureMSReason = "azure.messaging.reason"

	// OpenTelemetry attribute name for the Tracking ID of the operation
	attributeAzureMSTrackingID = "azure.messaging.tracking_id"

	// OpenTelemetry attribute name for Azure Messaging Application Group Name
	attributeAzureMSAppGroupName = "azure.messaging.application.group_name"
)

// azureMSCommon it's common struct for all Azure Messaging Audit logs,
// like ServiceBus, EventHub, etc. (AZMS*******Logs)
type azureMSCommon struct {
	EventTimestamp  string `json:"eventTimestamp"`
	EventTimeString string `json:"EventTimeString"`
	Environment     string `json:"Environment"`
	Region          string `json:"Region"`
	ScaleUnit       string `json:"ScaleUnit"`
	ActivityID      string `json:"ActivityId"`
	ActivityName    string `json:"ActivityName"`
	SubscriptionID  string `json:"SubscriptionId"`
	ResourceID      string `json:"ResourceId"`
	NamespaceName   string `json:"NamespaceName"`
	EntityType      string `json:"EntityType"`
	EntityName      string `json:"EntityName"`
}

func (r *azureMSCommon) GetResource() logsResourceAttributes {
	return logsResourceAttributes{
		ResourceID:      r.ResourceID,
		Location:        r.Region,
		Environment:     r.Environment,
		SubscriptionID:  r.SubscriptionID,
		SeviceNamespace: r.NamespaceName,
	}
}

func (r *azureMSCommon) GetTimestamp(formats ...string) (pcommon.Timestamp, error) {
	if r.EventTimestamp == "" && r.EventTimeString == "" {
		return pcommon.Timestamp(0), errNoTimestamp
	}

	time := r.EventTimestamp
	if time == "" {
		time = r.EventTimeString
	}

	nanos, err := unmarshaler.AsTimestamp(time, formats...)
	if err != nil {
		return pcommon.Timestamp(0), fmt.Errorf("unable to convert value %q as timestamp: %w", time, err)
	}

	return nanos, nil
}

func (*azureMSCommon) GetLevel() (plog.SeverityNumber, string, bool) {
	return plog.SeverityNumberUnspecified, "", false
}

func (r *azureMSCommon) PutCommonAttributes(attrs pcommon.Map, _ pcommon.Value) {
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSScaleUnit, r.ScaleUnit)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureActivityID, r.ActivityID)
	unmarshaler.AttrPutStrIf(attrs, unmarshaler.AttributeAzureOperationName, r.ActivityName)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSEntityName, r.EntityName)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSEntityType, r.EntityType)
}

func (*azureMSCommon) PutProperties(_ pcommon.Map, _ pcommon.Value) error {
	// By default - no "properties", so nothing to do here
	return nil
}

// See https://github.com/MicrosoftDocs/azure-docs/blob/main/articles/service-bus-messaging/monitor-service-bus-reference.md#diagnostic-error-logs
// and https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/azmsdiagnosticerrorlogs
// Available for microsoft.servicebus/namespaces and microsoft.eventhub/namespaces
type azureMSDiagnosticErrorLog struct {
	azureMSCommon

	TaskName string `json:"TaskName"`

	OperationResult string      `json:"OperationResult"`
	ErrorMessage    string      `json:"ErrorMessage"`
	ErrorCount      json.Number `json:"ErrorCount"` // int
}

func (*azureMSDiagnosticErrorLog) GetLevel() (plog.SeverityNumber, string, bool) {
	// Diagnostic Error logs are always Error level
	return plog.SeverityNumberError, "Error", true
}

func (r *azureMSDiagnosticErrorLog) PutCommonAttributes(attrs pcommon.Map, body pcommon.Value) {
	// Put common attributes first
	r.azureMSCommon.PutCommonAttributes(attrs, body)

	// Then put custom top-level attributes
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSTaskName, r.TaskName)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ErrorMessageKey), r.ErrorMessage)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeAzureMSErrorCount, r.ErrorCount)

	body.SetStr(r.OperationResult)
}

type azureMSApplicationMetricsLogProperties struct {
	ApplicationGroupName string `json:"ApplicationGroupName"`
}

func (p *azureMSApplicationMetricsLogProperties) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// This properties is actually an escaped JSON string,
	// so we need to unescape it first
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}

	// Define an alias type to avoid infinite recursion
	type alias azureMSApplicationMetricsLogProperties
	var temp alias

	if err := jsoniter.ConfigFastest.Unmarshal([]byte(s), &temp); err != nil {
		return err
	}

	// Assign the unmarshaled fields from the alias to the original struct
	*p = azureMSApplicationMetricsLogProperties(temp)

	return nil
}

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/azmsapplicationmetriclogs
// Available for microsoft.servicebus/namespaces and microsoft.eventhub/namespaces
type azureMSApplicationMetricsLog struct {
	azureMSCommon

	ChildEntityType string                                 `json:"ChildEntityType"`
	ChildEntityName string                                 `json:"ChildEntityName"`
	PartitionID     string                                 `json:"PartitionId"`
	Outcome         string                                 `json:"Outcome"`
	Protocol        string                                 `json:"Protocol"`
	AuthType        string                                 `json:"AuthType"`
	AuthID          string                                 `json:"AuthId"`
	NetworkType     string                                 `json:"NetworkType"`
	ClientIP        string                                 `json:"ClientIp"`
	Count           json.Number                            `json:"Count"` // int
	Properties      azureMSApplicationMetricsLogProperties `json:"Properties"`
}

func (r *azureMSApplicationMetricsLog) PutCommonAttributes(attrs pcommon.Map, body pcommon.Value) {
	// Put common attributes first
	r.azureMSCommon.PutCommonAttributes(attrs, body)

	// Then put custom top-level attributes
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSEntityChildType, r.ChildEntityType)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSEntityChildName, r.ChildEntityName)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSPartitionID, r.PartitionID)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkProtocolNameKey), strings.ToLower(r.Protocol))
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSAuthType, r.AuthType)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSAuthID, r.AuthID)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkConnectionTypeKey), r.NetworkType)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ClientAddressKey), r.ClientIP)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeAzureMSCount, r.Count)

	body.SetStr(r.Outcome)
}

func (r *azureMSApplicationMetricsLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSAppGroupName, r.Properties.ApplicationGroupName)

	return nil
}

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/azmsoperationallogs
// or https://github.com/MicrosoftDocs/azure-docs/blob/main/articles/service-bus-messaging/monitor-service-bus-reference.md#operational-logs
// Available for microsoft.servicebus/namespaces and microsoft.eventhub/namespaces
type azureMSOperationalLogProperties struct {
	SubscriptionID string `json:"SubscriptionId"`
	Namespace      string `json:"Namespace"`
	ViaURL         string `json:"Via"`
	TrackingID     string `json:"TrackingId"`
	ErrorCode      string `json:"ErrorCode"`
	ErrorMessage   string `json:"ErrorMessage"`
}

func (p *azureMSOperationalLogProperties) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// This properties is actually an escaped JSON string,
	// so we need to unescape it first
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}

	// Define an alias type to avoid infinite recursion
	type alias azureMSOperationalLogProperties
	var temp alias

	if err := jsoniter.ConfigFastest.Unmarshal([]byte(s), &temp); err != nil {
		return err
	}

	// Assign the unmarshaled fields from the alias to the original struct
	*p = azureMSOperationalLogProperties(temp)

	return nil
}

type azureMSOperationalLog struct {
	azureMSCommon

	EventName string `json:"EventName"`
	Status    string `json:"Status"`
	Caller    string `json:"Caller"`

	Properties azureMSOperationalLogProperties `json:"EventProperties"`
}

func (r *azureMSOperationalLog) PutCommonAttributes(attrs pcommon.Map, body pcommon.Value) {
	// Put common attributes first
	r.azureMSCommon.PutCommonAttributes(attrs, body)

	// Then put custom top-level attributes
	unmarshaler.AttrPutStrIf(attrs, attributeEventName, r.EntityName)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSStatus, r.Status)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSCaller, r.Caller)
}

func (r *azureMSOperationalLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	// SubscriptionId and Namespace are already in top-level attributes, so skip them here
	unmarshaler.AttrPutURLParsed(attrs, r.Properties.ViaURL)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSTrackingID, r.Properties.TrackingID)
	unmarshaler.AttrPutStrIf(attrs, attributeErrorCode, r.Properties.ErrorCode)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ErrorMessageKey), r.Properties.ErrorMessage)

	return nil
}

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/azmsruntimeauditlogs
// or https://github.com/MicrosoftDocs/azure-docs/blob/main/articles/service-bus-messaging/monitor-service-bus-reference.md#runtime-audit-logs
// Available for microsoft.servicebus/namespaces and microsoft.eventhub/namespaces
type azureMSRuntimeAuditLog struct {
	azureMSCommon

	TaskName    string      `json:"TaskName"`
	Status      string      `json:"Status"`
	Protocol    string      `json:"Protocol"`
	AuthType    string      `json:"AuthType"`
	AuthID      string      `json:"AuthId"`
	NetworkType string      `json:"NetworkType"`
	ClientIP    string      `json:"ClientIp"`
	Count       json.Number `json:"Count"`      // int
	Properties  string      `json:"Properties"` // unknown structure, save as is
}

func (r *azureMSRuntimeAuditLog) PutCommonAttributes(attrs pcommon.Map, body pcommon.Value) {
	// Put common attributes first
	r.azureMSCommon.PutCommonAttributes(attrs, body)

	// Then put custom top-level attributes
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSTaskName, r.TaskName)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSStatus, r.Status)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkProtocolNameKey), strings.ToLower(r.Protocol))
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSAuthType, r.AuthType)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSAuthID, r.AuthID)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkConnectionTypeKey), r.NetworkType)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ClientAddressKey), r.ClientIP)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeAzureMSCount, r.Count)
	// Put unparsed properties to log.Body as common approach
	body.SetStr(r.Properties)
}

type azureMSVNetAndIPFilteringLog struct {
	azureMSCommon

	EventName string      `json:"EventName"`
	IPAddress string      `json:"ipAddress"`
	Action    string      `json:"action"`
	Reason    string      `json:"reason"`
	Count     json.Number `json:"count"` // int
}

func (r *azureMSVNetAndIPFilteringLog) PutCommonAttributes(attrs pcommon.Map, body pcommon.Value) {
	// Put common attributes first
	r.azureMSCommon.PutCommonAttributes(attrs, body)

	// Then put custom top-level attributes
	unmarshaler.AttrPutStrIf(attrs, attributeEventName, r.EventName)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ClientAddressKey), r.IPAddress)
	unmarshaler.AttrPutStrIf(attrs, attributeSecurityRuleActionKey, r.Action)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureMSReason, r.Reason)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeAzureMSCount, r.Count)
}
