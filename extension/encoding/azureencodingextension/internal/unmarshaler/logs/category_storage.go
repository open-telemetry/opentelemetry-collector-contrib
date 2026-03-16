// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"encoding/json"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

const (
	// OpenTelemetry attribute name for Azure Storage account name
	attributeStorageAccountName = "azure.storage.namespace"

	// OpenTelemetry attribute name for the service associated with this request (blob, table, files or queue)
	attributeStorageServiceType = "azure.storage.service.type"

	// OpenTelemetry attribute name for the number of each logged operation that is involved in the request
	attributeStorageOperationCount = "azure.storage.operation.count"

	// OpenTelemetry attribute name for the key of the requested object
	attributeStorageObjectKey = "azure.storage.object.key"

	// OpenTelemetry attribute name for the source tier of the storage account
	attributeStorageSourceAccessTier = "azure.storage.source.access_tier"

	// OpenTelemetry attribute name for the size of the request header expressed in bytes
	attributeHTTPRequestHeaderSize = "http.request.header.size"

	// OpenTelemetry attribute name for the size of the response header expressed in bytes
	attributeHTTPResponseHeaderSize = "http.response.header.size"

	// OpenTelemetry attribute name for HTTP Response Status Text
	attributeHTTPResponseStatusText = "http.response.status_text"

	// OpenTelemetry attribute name for Azure HTTP Response Duration,
	// this is server-side duration of operation, excluding network time
	attributeAzureResponseDuration = "azure.response.duration"
)

// ------------------------------------------------------------
// Storage Log Identity
// ------------------------------------------------------------

// storageAuthorizationEntry represents a single authorization decision in Storage logs
type storageAuthorizationEntry struct {
	Action           string `json:"action"`
	RoleAssignmentID string `json:"roleAssignmentId"`
	RoleDefinitionID string `json:"roleDefinitionId"`
	DenyAssignmentID string `json:"denyAssignmentId"`
	Type             string `json:"type"`
	Result           string `json:"result"`
	Reason           string `json:"reason"`
	Principals       []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"principals"`
}

// storageRequester represents the requester information in Storage logs
type storageRequester struct {
	ObjectID string `json:"objectId"`
	TenantID string `json:"tenantId"`
}

// azureIdentityStorage describes the identity for Storage Blob Logs.
// Unlike Activity Logs, Storage identity represents an authorization decision audit
// with `authorization` as an array and additional fields like `type` and `tokenHash`.
//
// Embeds azureIdentityBase to follow the same structural pattern as azureIdentityActivity.
// Storage logs don't have JWT claims, so the base claims extraction is a no-op.
type azureIdentityStorage struct {
	azureIdentityBase

	Type          string                      `json:"type"`
	TokenHash     string                      `json:"tokenHash"`
	Authorization []storageAuthorizationEntry `json:"authorization"`
	Requester     *storageRequester           `json:"requester"`
}

// PutIdentityAttributes extracts storage identity fields into OTel attributes.
// Calls the base method first (for common claims, if any), then adds
// storage-specific fields as a nested map under `azure.identity`.
func (id *azureIdentityStorage) PutIdentityAttributes(attrs pcommon.Map) {
	// Common identity fields (claims) - no-op for Storage logs since they
	// don't have JWT claims, but keeps the pattern consistent with Activity logs.
	id.azureIdentityBase.PutIdentityAttributes(attrs)

	// Storage-specific identity fields as nested map
	identityMap := attrs.PutEmptyMap(attributeAzureIdentity)

	unmarshaler.AttrPutStrIf(identityMap, "type", id.Type)
	unmarshaler.AttrPutStrIf(identityMap, "tokenHash", id.TokenHash)

	if len(id.Authorization) > 0 {
		authSlice := identityMap.PutEmptySlice("authorization")
		for i := range id.Authorization {
			entry := &id.Authorization[i]
			authMap := authSlice.AppendEmpty().SetEmptyMap()
			unmarshaler.AttrPutStrIf(authMap, "action", entry.Action)
			unmarshaler.AttrPutStrIf(authMap, "roleAssignmentId", entry.RoleAssignmentID)
			unmarshaler.AttrPutStrIf(authMap, "roleDefinitionId", entry.RoleDefinitionID)
			unmarshaler.AttrPutStrIf(authMap, "denyAssignmentId", entry.DenyAssignmentID)
			unmarshaler.AttrPutStrIf(authMap, "type", entry.Type)
			unmarshaler.AttrPutStrIf(authMap, "result", entry.Result)
			unmarshaler.AttrPutStrIf(authMap, "reason", entry.Reason)

			if len(entry.Principals) > 0 {
				principalsSlice := authMap.PutEmptySlice("principals")
				for j := range entry.Principals {
					p := &entry.Principals[j]
					pMap := principalsSlice.AppendEmpty().SetEmptyMap()
					unmarshaler.AttrPutStrIf(pMap, "id", p.ID)
					unmarshaler.AttrPutStrIf(pMap, "type", p.Type)
				}
			}
		}
	}

	if id.Requester != nil {
		requesterMap := identityMap.PutEmptyMap("requester")
		unmarshaler.AttrPutStrIf(requesterMap, "objectId", id.Requester.ObjectID)
		unmarshaler.AttrPutStrIf(requesterMap, "tenantId", id.Requester.TenantID)
	}
}

// ------------------------------------------------------------
// Storage Blob Log Category
// ------------------------------------------------------------

// See https://github.com/MicrosoftDocs/azure-docs/blob/main/includes/azure-storage-logs-properties-service.md
// All categories, like StorageRead, StorageWrite, StorageDelete share the same properties,
// called StorageBlobLogs, see https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/storagebloblogs
type azureStorageBlobLog struct {
	azureLogRecordBase

	// Identity is parsed directly into azureIdentityStorage during the
	// main JSON unmarshal step - no double parsing required.
	Identity *azureIdentityStorage `json:"identity"`

	// Additional fields in common schema
	StatusCode *json.Number `json:"statusCode"` // int
	StatusText *string      `json:"statusText"`
	URI        *string      `json:"uri"`
	Protocol   *string      `json:"protocol"`

	Properties struct {
		AccountName        string      `json:"accountName"`
		UserAgentHeader    string      `json:"userAgentHeader"`
		ClientRequestID    string      `json:"clientRequestId"`
		ServerLatencyMs    json.Number `json:"serverLatencyMs"` // float
		ServiceType        string      `json:"serviceType"`
		OperationCount     json.Number `json:"operationCount"`     // int
		RequestHeaderSize  json.Number `json:"requestHeaderSize"`  // int
		RequestBodySize    json.Number `json:"requestBodySize"`    // int
		ResponseHeaderSize json.Number `json:"responseHeaderSize"` // int
		ResponseBodySize   json.Number `json:"responseBodySize"`   // int
		TLSVersion         string      `json:"tlsVersion"`
		ObjectKey          string      `json:"objectKey"`
		SourceAccessTier   string      `json:"sourceAccessTier"`
	} `json:"properties"`
}

func (r *azureStorageBlobLog) PutCommonAttributes(attrs pcommon.Map, body pcommon.Value) {
	// Put common attributes first
	r.azureLogRecordBase.PutCommonAttributes(attrs, body)

	// Storage identity is semantically different from Activity Log identity
	// (authorization audit vs caller identity). Parse into typed structure.
	if r.Identity != nil {
		r.Identity.PutIdentityAttributes(attrs)
	}

	// Then put custom top-level attributes
	// `StatusCode` might be set to "Unknown" value according to Azure docs
	if r.StatusCode != nil && r.StatusCode.String() != "Unknown" {
		unmarshaler.AttrPutIntNumberPtrIf(attrs, string(conventions.HTTPResponseStatusCodeKey), r.StatusCode)
	}
	unmarshaler.AttrPutStrPtrIf(attrs, attributeHTTPResponseStatusText, r.StatusText)
	if r.Protocol != nil {
		unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkProtocolNameKey), strings.ToLower(*r.Protocol))
	}
	if r.URI != nil {
		unmarshaler.AttrPutURLParsed(attrs, *r.URI)
	}
}

func (r *azureStorageBlobLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, attributeStorageAccountName, r.Properties.AccountName)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.UserAgentOriginalKey), r.Properties.UserAgentHeader)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.AzureServiceRequestIDKey), r.Properties.ClientRequestID)
	unmarshaler.AttrPutFloatNumberIf(attrs, attributeAzureResponseDuration, r.Properties.ServerLatencyMs)
	unmarshaler.AttrPutStrIf(attrs, attributeStorageServiceType, r.Properties.ServiceType)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeStorageOperationCount, r.Properties.OperationCount)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.HTTPRequestBodySizeKey), r.Properties.RequestBodySize)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeHTTPRequestHeaderSize, r.Properties.RequestHeaderSize)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.HTTPResponseBodySizeKey), r.Properties.ResponseBodySize)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeHTTPResponseHeaderSize, r.Properties.RequestHeaderSize)
	attrPutTLSProtoIf(attrs, r.Properties.TLSVersion)
	unmarshaler.AttrPutStrIf(attrs, attributeStorageObjectKey, r.Properties.ObjectKey)
	unmarshaler.AttrPutStrIf(attrs, attributeStorageSourceAccessTier, r.Properties.SourceAccessTier)

	return nil
}
