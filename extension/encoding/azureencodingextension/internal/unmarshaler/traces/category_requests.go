// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/traces"

import (
	"encoding/json"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

const (
	// OpenTelemetry attribute name for Azure Friendly name of the request source,
	// from `Source` field in Azure Trace Record
	attributeAzureRequestSource = "azure.request.source"
)

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/apprequests
type azureAppRequests struct {
	azureTracesRecordBase

	// AppRequests only fields
	ResultCode *json.Number `json:"ResultCode"` // Result code returned by or to the application
	Source     string       `json:"Source"`     // Friendly name of the request source, when known. Source is based on the metadata supplied by the caller
	URL        string       `json:"Url"`        // URL of the request
}

// GetSpanKind determines the SpanKind based on the provided URL
func (r *azureAppRequests) GetSpanKind() ptrace.SpanKind {
	// If URL is set - we can assume that it's "Server" SpanKind
	if r.URL != "" {
		return ptrace.SpanKindServer
	}

	// By default - returns "Internal" Span Kind as of OpenTelemetry SemConv Spec
	return ptrace.SpanKindInternal
}

// PutCommonAttributes puts already parsed common attributes into provided Attributes Map/Body
func (r *azureAppRequests) PutCommonAttributes(attrs pcommon.Map) {
	r.azureTracesRecordBase.PutCommonAttributes(attrs)

	unmarshaler.AttrPutStrIf(attrs, attributeAzureRequestSource, r.Source) // unstable SemConv
	unmarshaler.AttrPutURLParsed(attrs, r.URL)
	// If ResultCode AND URL are set - put status code to the attributes
	if r.URL != "" {
		unmarshaler.AttrPutIntNumberPtrIf(attrs, string(conventions.HTTPResponseStatusCodeKey), r.ResultCode)
	}
}
