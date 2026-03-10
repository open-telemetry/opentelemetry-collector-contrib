// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/traces"

import (
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appavailabilityresults
type azureAppAvailabilityResults struct {
	azureTracesRecordBase

	// AppAvailabilityResults only fields
	Location string `json:"Location"` // The location from where the test ran
	Message  string `json:"Message"`  // Application-defined message
}

// GetResource returns resource attributes for the parsed Trace Record,
// adding cloud.region from `Location` field
func (r *azureAppAvailabilityResults) GetResource() traceResourceAttributes {
	rs := r.azureTracesRecordBase.GetResource()
	rs.Location = r.Location

	return rs
}

// GetSpanKind determines the SpanKind
func (*azureAppAvailabilityResults) GetSpanKind() ptrace.SpanKind {
	// As we unsure about AvailabilityResults SpanKind - set it to Internal
	return ptrace.SpanKindInternal
}

// GetSpanStatus returns Span Status Code and optional Status Message,
// based on `Success` field
func (r *azureAppAvailabilityResults) GetSpanStatus() (ptrace.StatusCode, string) {
	// According to Azure Docs if `Success` is false - the operation failed,
	// so we'll mark Span Status as Error for such cases according to OpenTelemetry Specs
	if !r.Success {
		return ptrace.StatusCodeError, r.Message
	}

	// In all other cases - return Unset Status Code as recommended by OpenTelemetry Specs
	return ptrace.StatusCodeUnset, r.Message
}
