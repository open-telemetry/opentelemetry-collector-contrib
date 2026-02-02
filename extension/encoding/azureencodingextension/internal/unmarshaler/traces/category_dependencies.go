// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/traces"

import (
	"encoding/json"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

const (
	// OpenTelemetry attribute name for Azure detailed information about the dependency call,
	// from `Data` field in Azure Trace Record
	attributeAzureDependencyData = "azure.dependency.data"

	// OpenTelemetry attribute name for Azure dependency type, such as "HTTP" or "SQL",
	// from `DependencyType` field in Azure Trace Record
	attributeAzureDependencyType = "azure.dependency.type"

	// OpenTelemetry attribute name for Azure target of a dependency call, such as a Web or a SQL server name,
	// from `Target` field in Azure Trace Record
	attributeAzureDependencyTarget = "azure.dependency.target"
)

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appdependencies
type azureAppDependencies struct {
	azureTracesRecordBase

	// AppDependencies only fields
	ResultCode     *json.Number `json:"ResultCode"`     // Result code returned by or to the application
	Data           string       `json:"Data"`           // Detailed information about the dependency call, such as a full URI or a SQL statement
	DependencyType string       `json:"DependencyType"` // Dependency type, such as HTTP or SQL
	Target         string       `json:"Target"`         // Target of a dependency call, such as a Web or a SQL server name
}

// GetSpanKind determines the SpanKind based on the Azure Dependency Type
func (r *azureAppDependencies) GetSpanKind() ptrace.SpanKind {
	// `DependencyType` == "Queue Message | <name of messaging system>" => Producer Span Kind
	if strings.HasPrefix(r.DependencyType, "Queue") {
		return ptrace.SpanKindProducer
	}

	// `Data` field is set - we can assume that it's "Client" SpanKind,
	// as according to Azure docs - `Data` field contains URI/DB Statement
	if r.Data != "" {
		return ptrace.SpanKindClient
	}

	// By default - returns "Internal" Span Kind as of OpenTelemetry SemConv Spec
	return ptrace.SpanKindInternal
}

// PutCommonAttributes puts already parsed common attributes into provided Attributes Map/Body
func (r *azureAppDependencies) PutCommonAttributes(attrs pcommon.Map) {
	r.azureTracesRecordBase.PutCommonAttributes(attrs)

	unmarshaler.AttrPutStrIf(attrs, attributeAzureDependencyType, r.DependencyType)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureDependencyTarget, r.Target)

	// AppDependencies doesn't have "URL" field, it is using "Data" field instead
	if r.Data != "" && strings.HasPrefix(r.Data, "http") {
		unmarshaler.AttrPutURLParsed(attrs, r.Data)
		unmarshaler.AttrPutIntNumberPtrIf(attrs, string(conventions.HTTPResponseStatusCodeKey), r.ResultCode)
	} else {
		// Otherwise, we will save Data field as is
		unmarshaler.AttrPutStrIf(attrs, attributeAzureDependencyData, r.Data)
	}
}
