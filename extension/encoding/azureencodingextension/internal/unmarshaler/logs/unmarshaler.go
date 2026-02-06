// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"bufio"
	"bytes"
	"fmt"
	"time"

	gojson "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

// Commonly used non-SemConv attributes
const (
	// Predefined value for `cloudevents.event_source` resource attribute
	attributeCloudEventSourceValue = "azure.resource.log"
	// OpenTelemetry resource attribute name for Azure Tenant ID
	attributeAzureTenantID = "azure.tenant.id"
)

// logsResourceAttributes is a helper struct to hold resource attributes for specific log records
// Each Category Parser decides which Resource Attributes to populate
type logsResourceAttributes struct {
	ResourceID        string
	TenantID          string
	SubscriptionID    string
	Location          string
	SeviceNamespace   string
	ServiceName       string
	ServiceInstanceID string
	Environment       string
}

// categoryHolder is a small helper struct to get `category`/`type` fields
// from each Log Record
type categoryHolder struct {
	Category string `json:"category"`
	Type     string `json:"type"` // Used in AppInsights logs instead of `category`
}

type ResourceLogsUnmarshaler struct {
	buildInfo         component.BuildInfo
	logger            *zap.Logger
	timeFormat        []string
	includeCategories map[string]bool
	excludeCategories map[string]bool
	hasIncludes       bool
}

func (r ResourceLogsUnmarshaler) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	allResourceScopeLogs := map[logsResourceAttributes]plog.ScopeLogs{}

	batchFormat, err := unmarshaler.DetectWrapperFormat(buf)
	if err != nil {
		return plog.NewLogs(), err
	}

	switch batchFormat {
	// ND JSON is a specific case...
	// We will use bufio.Scanner trick to read it line by line
	// as unmarshal each line as a Log Record
	case unmarshaler.FormatNDJSON:
		scanner := bufio.NewScanner(bytes.NewReader(buf))
		for scanner.Scan() {
			r.unmarshalRecord(allResourceScopeLogs, scanner.Bytes())
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
			return plog.NewLogs(), fmt.Errorf("failed to create JSON Path %q: %w", jsonPath, err)
		}

		records, err := rootPath.Extract(buf)
		if err != nil {
			// This should never happen, but still...
			return plog.NewLogs(), fmt.Errorf("failed to extract Azure Log Records: %w", err)
		}

		for _, record := range records {
			r.unmarshalRecord(allResourceScopeLogs, record)
		}
	// This is happened on empty input
	case unmarshaler.FormatUnknown:
		return plog.NewLogs(), nil
	default:
	}

	l := plog.NewLogs()
	for resourceID, scopeLogs := range allResourceScopeLogs {
		rl := l.ResourceLogs().AppendEmpty()
		ra := rl.Resource().Attributes()
		ra.EnsureCapacity(10)
		// Set SemConv attributes
		unmarshaler.AttrPutStrIf(ra, string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())
		unmarshaler.AttrPutStrIf(ra, string(conventions.CloudEventsEventSourceKey), attributeCloudEventSourceValue)
		// Resource attributes parsed by Category Parser
		unmarshaler.AttrPutStrIf(ra, string(conventions.CloudResourceIDKey), resourceID.ResourceID)
		unmarshaler.AttrPutStrIf(ra, string(conventions.CloudRegionKey), resourceID.Location)
		unmarshaler.AttrPutStrIf(ra, string(conventions.ServiceNamespaceKey), resourceID.SeviceNamespace)
		unmarshaler.AttrPutStrIf(ra, string(conventions.ServiceNameKey), resourceID.ServiceName)
		unmarshaler.AttrPutStrIf(ra, string(conventions.ServiceInstanceIDKey), resourceID.ServiceInstanceID)
		unmarshaler.AttrPutStrIf(ra, string(conventions.DeploymentEnvironmentNameKey), resourceID.Environment)
		unmarshaler.AttrPutStrIf(ra, attributeAzureTenantID, resourceID.TenantID)
		// In Azure - Subscription is the closes analog to the Account,
		// so we'll transform SubscriptionID into `cloud.account.id`
		unmarshaler.AttrPutStrIf(ra, string(conventions.CloudAccountIDKey), resourceID.SubscriptionID)
		scopeLogs.MoveTo(rl.ScopeLogs().AppendEmpty())
	}

	return l, nil
}

func (r ResourceLogsUnmarshaler) unmarshalRecord(allResourceScopeLogs map[logsResourceAttributes]plog.ScopeLogs, record []byte) {
	// Despite of the fact that official Azure documentation states that exists common Logs schema
	// (see https://learn.microsoft.com/en-us/azure/azure-monitor/platform/resource-logs-schema),
	// in reality - it's not true, some Resources exposing Logs in totally different formats.
	// For example, Azure Service Bus logs doesn't conform schema above
	// (see examples here - https://github.com/noakup/AzMonLogsAgent/blob/main/NGSchema/AzureServiceBus/SampleInputRecords/ServiceBusOperationLogSample.json)
	// The only 2 fields that SHOULD be present in most Log schemas - `category` and `resourceId`
	// So, proper way to correctly decode incoming Log Record - first get value from `category` field
	// and Unmarshal record into category-specific struct.
	// That's actually double-unmarshaling, but there is no other way to parse variety of Azure Logs schemas
	var ch categoryHolder
	if err := jsoniter.ConfigFastest.Unmarshal(record, &ch); err != nil {
		r.logger.Error("JSON unmarshal failed for Azure Log Record", zap.Error(err))
		return
	}
	logCategory := ch.Category
	if logCategory == "" {
		logCategory = ch.Type
	}

	if logCategory == "" {
		// We couldn't do any SemConv conversion as it's an unknown Log Schema for us,
		// because it doesn't have a "category" field which we rely on
		// So we will save incoming Log Record as a JSON string into Body just
		// not to loose data
		r.logger.Warn(
			"No Category field are set on Log Record, couldn't parse SemConv way, will save it as-is",
		)
		r.storeRawLog(allResourceScopeLogs, record)
		return
	}

	// Filter out categories based on provided configuration
	if _, exclude := r.excludeCategories[logCategory]; exclude {
		return
	}
	if r.hasIncludes {
		if _, include := r.includeCategories[logCategory]; !include {
			return
		}
	}

	// Let's parse it
	log, err := processLogRecord(logCategory, record)
	if err != nil {
		r.storeRawLog(allResourceScopeLogs, record)
		r.logger.Warn(
			"Unable to parse Log Record",
			zap.String("category", logCategory),
			zap.Error(err),
		)
		return
	}

	// Get timestamp for any of the possible fields
	nanos, err := log.GetTimestamp(r.timeFormat...)
	if err != nil {
		r.storeRawLog(allResourceScopeLogs, record)
		r.logger.Warn(
			"Unable to convert timestamp from log",
			zap.String("category", logCategory),
			zap.Error(err),
		)
		return
	}

	rs := log.GetResource()
	if rs.ResourceID == "" {
		r.logger.Warn(
			"No ResourceID set on Log record",
			zap.String("category", logCategory),
		)
	}
	scopeLogs := r.getScopeLog(allResourceScopeLogs, rs)

	lr := scopeLogs.LogRecords().AppendEmpty()
	lr.SetTimestamp(nanos)

	severity, severityName, isSet := log.GetLevel()
	// Do not set Log Severity if it's not provided in the Log Record
	// to avoid confusion with actual SeverityNumberUnspecified value
	if isSet {
		lr.SetSeverityNumber(severity)
		lr.SetSeverityText(severityName)
	}

	attrs := lr.Attributes()
	// Put Log Category anyway
	unmarshaler.AttrPutStrIf(attrs, unmarshaler.AttributeAzureCategory, logCategory)
	// Parse Common Attributes + Properties (if applicable)
	body := lr.Body()
	log.PutCommonAttributes(attrs, body)
	if err := log.PutProperties(attrs, body); err != nil {
		r.logger.Warn(
			"Unable to parse Azure Log Properties into OpenTelemetry Attributes",
			zap.String("category", logCategory),
			zap.String("resourceId", rs.ResourceID),
			zap.Error(err),
		)
	}
}

// getScopeLog gets current ScopeLog based on provided set of ResourceAttributes
// If ScopeLog doesn't exists - create a new one and append to the list
func (r ResourceLogsUnmarshaler) getScopeLog(allResourceScopeLogs map[logsResourceAttributes]plog.ScopeLogs, rs logsResourceAttributes) plog.ScopeLogs {
	scopeLogs, found := allResourceScopeLogs[rs]
	if !found {
		scopeLogs = plog.NewScopeLogs()
		scopeLogs.Scope().SetName(metadata.ScopeName)
		scopeLogs.Scope().SetVersion(r.buildInfo.Version)
		allResourceScopeLogs[rs] = scopeLogs
	}

	return scopeLogs
}

// storeRawLog stores incoming Azure Resource Log Record as a string into log.Body
// It's used we couldn't do any SemConv conversion, for example:
// * In case when there is no "category" field
// * In case when JSON unmarshaling failed
// Stored record than can be used for debugging and fixing purposes
func (r ResourceLogsUnmarshaler) storeRawLog(allResourceScopeLogs map[logsResourceAttributes]plog.ScopeLogs, record []byte) {
	// We couldn't do any SemConv conversion as it's an unknown Log Schema for us,
	// because it doesn't have a "category" field which we rely on
	// So we will save incoming Log Record as a JSON string into Body just
	// not to loose data
	scopeLogs := r.getScopeLog(allResourceScopeLogs, logsResourceAttributes{})
	lr := scopeLogs.LogRecords().AppendEmpty()
	// We couldn't get timestamp from incoming Record, so to keep the Log
	// we will set timestamp to current time
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	// Set unspecified log level
	lr.SetSeverityNumber(plog.SeverityNumberUnspecified)
	lr.SetSeverityText(plog.SeverityNumberUnspecified.String())
	// Put record to Body as-is
	lr.Body().SetStr(string(record))
}

func NewAzureResourceLogsUnmarshaler(buildInfo component.BuildInfo, logger *zap.Logger, cfg LogsConfig) ResourceLogsUnmarshaler {
	includeCategories := make(map[string]bool, len(cfg.IncludeCategories))
	for _, icat := range cfg.IncludeCategories {
		includeCategories[icat] = true
	}
	excludeCategories := make(map[string]bool, len(cfg.ExcludeCategories))
	for _, ecat := range cfg.ExcludeCategories {
		excludeCategories[ecat] = true
	}

	return ResourceLogsUnmarshaler{
		buildInfo:         buildInfo,
		logger:            logger,
		timeFormat:        cfg.TimeFormats,
		includeCategories: includeCategories,
		excludeCategories: excludeCategories,
		hasIncludes:       len(includeCategories) > 0,
	}
}
