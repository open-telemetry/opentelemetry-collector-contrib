// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/traces"

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	gojson "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

// categoryHolder is a small helper struct to get Category (`type`) fields
// from each Trace Record
type categoryHolder struct {
	Category string `json:"type"` // Used in AppInsights instead of `category` field
}

// traceResourceAttributes is a helper struct to hold resource attributes
type traceResourceAttributes struct {
	ResourceID      string
	ServiceName     string
	ServiceInstance string
	ServiceVersion  string
	SDKVersion      string
	Location        string
}

type ResourceTracesUnmarshaler struct {
	buildInfo  component.BuildInfo
	logger     *zap.Logger
	timeFormat []string
}

func (r ResourceTracesUnmarshaler) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	allResourceScopeSpans := map[traceResourceAttributes]ptrace.ScopeSpans{}

	batchFormat, err := unmarshaler.DetectWrapperFormat(buf)
	if err != nil {
		return ptrace.NewTraces(), err
	}

	switch batchFormat {
	// ND JSON is a specific case...
	// We will use bufio.Scanner trick to read it line by line
	// as unmarshal each line as a Log Record
	case unmarshaler.FormatNDJSON:
		scanner := bufio.NewScanner(bytes.NewReader(buf))
		for scanner.Scan() {
			r.unmarshalRecord(allResourceScopeSpans, scanner.Bytes())
		}
	// Both formats are valid JSON and can be parsed directly
	// `gojson.Path.Extract` is a bit faster and use ~25% less bytes per operation
	// comparing to unmarshaling to intermediate structure (e.g. using `var recordsHolder []json.RawMessage`)
	case unmarshaler.FormatObjectRecords, unmarshaler.FormatJSONArray:
		jsonPath := unmarshaler.JSONPathEventHubLogRecords
		if batchFormat == unmarshaler.FormatJSONArray {
			jsonPath = unmarshaler.JSONPathBlobStorageLogRecords
		}

		// This will allow us to parse Azure Log Records in both formats:
		// 1) As exported to Azure Event Hub, e.g. `{"records": [ {...}, {...} ]}`
		// 2) As exported to Azure Blob Storage, e.g. `[ {...}, {...} ]`
		rootPath, err := gojson.CreatePath(jsonPath)
		if err != nil {
			// This should never happen, but still...
			return ptrace.NewTraces(), fmt.Errorf("failed to create JSON Path %q: %w", jsonPath, err)
		}

		records, err := rootPath.Extract(buf)
		if err != nil {
			// This should never happen, but still...
			return ptrace.NewTraces(), fmt.Errorf("failed to extract Azure Trace Records: %w", err)
		}

		for _, record := range records {
			r.unmarshalRecord(allResourceScopeSpans, record)
		}
	// This is happened on empty input
	case unmarshaler.FormatUnknown:
		return ptrace.NewTraces(), nil
	default:
	}

	t := ptrace.NewTraces()
	for resourceAttrs, scopeSpans := range allResourceScopeSpans {
		rs := t.ResourceSpans().AppendEmpty()
		rsa := rs.Resource().Attributes()
		rsa.EnsureCapacity(8) // 6 pre-defined attributes + 2 for SDK name/version
		unmarshaler.AttrPutStrIf(rsa, string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())
		unmarshaler.AttrPutStrIf(rsa, string(conventions.CloudResourceIDKey), resourceAttrs.ResourceID)
		unmarshaler.AttrPutStrIf(rsa, string(conventions.ServiceNameKey), resourceAttrs.ServiceName)
		unmarshaler.AttrPutStrIf(rsa, string(conventions.ServiceInstanceIDKey), resourceAttrs.ServiceInstance)
		unmarshaler.AttrPutStrIf(rsa, string(conventions.ServiceVersionKey), resourceAttrs.ServiceVersion)
		unmarshaler.AttrPutStrIf(rsa, string(conventions.CloudRegionKey), resourceAttrs.Location)
		// SDK Name and Version parsing
		if resourceAttrs.SDKVersion != "" {
			parts := strings.Split(resourceAttrs.SDKVersion, ":")
			if len(parts) == 2 {
				unmarshaler.AttrPutStrIf(rsa, string(conventions.TelemetrySDKNameKey), strings.TrimSpace(parts[0]))
				unmarshaler.AttrPutStrIf(rsa, string(conventions.TelemetrySDKVersionKey), strings.TrimSpace(parts[1]))
			} else {
				// Unparsable SDK string - save as is
				unmarshaler.AttrPutStrIf(rsa, string(conventions.TelemetrySDKNameKey), resourceAttrs.SDKVersion)
			}
		}
		scopeSpans.MoveTo(rs.ScopeSpans().AppendEmpty())
	}

	return t, nil
}

func (r ResourceTracesUnmarshaler) unmarshalRecord(allResourceScopeSpans map[traceResourceAttributes]ptrace.ScopeSpans, record []byte) {
	// That's actually double-unmarshaling, but there is no other way to parse variety of Azure Resource schemas
	var ch categoryHolder
	if err := jsoniter.ConfigFastest.Unmarshal(record, &ch); err != nil {
		r.logger.Error("JSON unmarshal failed for Azure Trace Record", zap.Error(err))
		return
	}

	// We couldn't do any SemConv conversion if Schema Category is not defined
	if ch.Category == "" {
		r.logger.Error("No Category field are set on Trace Record, couldn't parse")
		return
	}

	// Unsupported Category Type - skip processing with Warning
	if ch.Category != categoryAvailabilityResults &&
		ch.Category != categoryDependencies &&
		ch.Category != categoryRequests {
		r.logger.Warn(
			"Unsupported Category Type",
			zap.String("Category", ch.Category),
		)
		return
	}

	// Let's parse it
	traceRecord, err := processTraceRecord(ch.Category, record)
	if err != nil {
		r.logger.Warn(
			"Unable to parse Trace Record",
			zap.String("category", ch.Category),
			zap.Error(err),
		)
		return
	}

	// Get timestamp for start time (and end time calculation)
	nanos, err := traceRecord.GetTimestamp(r.timeFormat...)
	if err != nil {
		r.logger.Warn(
			"Unable to convert timestamp from Trace Record",
			zap.String("category", ch.Category),
			zap.Error(err),
		)
		return
	}

	traceID, err := traceRecord.GetTraceID()
	if err != nil {
		r.logger.Warn(
			"Unable to convert TraceID from Trace Record",
			zap.String("category", ch.Category),
			zap.Error(err),
		)
		return
	}

	spanID, err := traceRecord.GetSpanID()
	if err != nil {
		r.logger.Warn(
			"Unable to convert SpanID from Trace Record",
			zap.String("category", ch.Category),
			zap.Error(err),
		)
		return
	}

	parentSpanID, err := traceRecord.GetParentSpanID()
	if err != nil {
		r.logger.Warn(
			"Unable to convert ParentSpanID from Trace Record",
			zap.String("category", ch.Category),
			zap.Error(err),
		)
	}

	// Sometimes, ResourceID is not enough to uniquely identify a resource in Azure cloud
	// For example, multiple Azure Functions deployed into single Azure Functions App,
	// will be sharing the same ResourceID,
	// but actually it's different Resources with own ServiceName, ServiceInstance and ServiceVersion.
	rs := traceRecord.GetResource()
	if rs.ResourceID == "" {
		r.logger.Warn(
			"No ResourceID set on Trace record",
			zap.String("OperationId", traceID.String()),
			zap.String("Id", spanID.String()),
		)
	}

	// Grouping set of traces by Resource Attributes
	scopeSpans, found := allResourceScopeSpans[rs]
	if !found {
		scopeSpans = ptrace.NewScopeSpans()
		scopeSpans.Scope().SetName(metadata.ScopeName)
		scopeSpans.Scope().SetVersion(r.buildInfo.Version)
		allResourceScopeSpans[rs] = scopeSpans
	}

	// Populate Span data
	span := scopeSpans.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetKind(traceRecord.GetSpanKind())
	span.SetName(traceRecord.GetSpanName())
	span.SetStartTimestamp(nanos)
	span.SetEndTimestamp(nanos + traceRecord.GetSpanDuration())
	statusCode, statusMessage := traceRecord.GetSpanStatus()
	span.Status().SetCode(statusCode)
	if statusMessage != "" {
		span.Status().SetMessage(statusMessage)
	}

	// Populate Span Attributes
	attrs := span.Attributes()
	traceRecord.PutCommonAttributes(attrs)
	traceRecord.PutProperties(attrs)
}

func NewAzureResourceTracesUnmarshaler(buildInfo component.BuildInfo, logger *zap.Logger, cfg TracesConfig) ResourceTracesUnmarshaler {
	return ResourceTracesUnmarshaler{
		buildInfo:  buildInfo,
		logger:     logger,
		timeFormat: cfg.TimeFormats,
	}
}
