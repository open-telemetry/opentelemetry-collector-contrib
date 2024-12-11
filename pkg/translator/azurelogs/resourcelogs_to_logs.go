// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/relvacode/iso8601"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

const (
	// Constants for OpenTelemetry Specs
	scopeName = "otelcol/azureresourcelogs"

	// Constants for Azure Log Record Attributes
	// TODO: Remove once these are available in semconv
	eventName          = "event.name"
	eventNameValue     = "az.resource.log"
	networkPeerAddress = "network.peer.address"

	// Constants for Azure Log Record body fields
	azureCategory          = "category"
	azureCorrelationID     = "correlation.id"
	azureDuration          = "duration"
	azureIdentity          = "identity"
	azureOperationName     = "operation.name"
	azureOperationVersion  = "operation.version"
	azureProperties        = "properties"
	azureResultType        = "result.type"
	azureResultSignature   = "result.signature"
	azureResultDescription = "result.description"
	azureTenantID          = "tenant.id"
)

var errMissingTimestamp = errors.New("missing timestamp")

// as exported via an Azure Event Hub
type azureRecords struct {
	Records []azureLogRecord `json:"records"`
}

// azureLogRecord represents a single Azure log following
// the common schema:
// https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/resource-logs-schema
type azureLogRecord struct {
	Time              string       `json:"time"`
	Timestamp         string       `json:"timeStamp"`
	ResourceID        string       `json:"resourceId"`
	TenantID          *string      `json:"tenantId"`
	OperationName     string       `json:"operationName"`
	OperationVersion  *string      `json:"operationVersion"`
	Category          string       `json:"category"`
	ResultType        *string      `json:"resultType"`
	ResultSignature   *string      `json:"resultSignature"`
	ResultDescription *string      `json:"resultDescription"`
	DurationMs        *json.Number `json:"durationMs"`
	CallerIPAddress   *string      `json:"callerIpAddress"`
	CorrelationID     *string      `json:"correlationId"`
	Identity          *any         `json:"identity"`
	Level             *json.Number `json:"Level"`
	Location          *string      `json:"location"`
	Properties        *any         `json:"properties"`
}

var _ plog.Unmarshaler = (*ResourceLogsUnmarshaler)(nil)

type ResourceLogsUnmarshaler struct {
	Version    string
	Logger     *zap.Logger
	TimeFormat []string
}

func (r ResourceLogsUnmarshaler) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	l := plog.NewLogs()

	var azureLogs azureRecords
	decoder := jsoniter.NewDecoder(bytes.NewReader(buf))
	if err := decoder.Decode(&azureLogs); err != nil {
		return l, err
	}

	var resourceIDs []string
	azureResourceLogs := make(map[string][]azureLogRecord)
	for _, azureLog := range azureLogs.Records {
		azureResourceLogs[azureLog.ResourceID] = append(azureResourceLogs[azureLog.ResourceID], azureLog)
		keyExists := slices.Contains(resourceIDs, azureLog.ResourceID)
		if !keyExists {
			resourceIDs = append(resourceIDs, azureLog.ResourceID)
		}
	}

	for _, resourceID := range resourceIDs {
		logs := azureResourceLogs[resourceID]
		resourceLogs := l.ResourceLogs().AppendEmpty()
		scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
		scopeLogs.Scope().SetName(scopeName)
		scopeLogs.Scope().SetVersion(r.Version)
		logRecords := scopeLogs.LogRecords()

		for i := 0; i < len(logs); i++ {
			log := logs[i]
			nanos, err := getTimestamp(log, r.TimeFormat)
			if err != nil {
				r.Logger.Warn("Unable to convert timestamp from log", zap.String("timestamp", log.Time))
				continue
			}

			lr := logRecords.AppendEmpty()
			lr.SetTimestamp(nanos)

			if log.Level != nil {
				severity := asSeverity(*log.Level)
				lr.SetSeverityNumber(severity)
				lr.SetSeverityText(log.Level.String())
			}

			lr.Attributes().PutStr(conventions.AttributeCloudResourceID, resourceID)
			lr.Attributes().PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
			lr.Attributes().PutStr(eventName, eventNameValue)

			if err := lr.Body().FromRaw(extractRawAttributes(log)); err != nil {
				return l, err
			}
		}
	}

	return l, nil
}

func getTimestamp(record azureLogRecord, formats []string) (pcommon.Timestamp, error) {
	if record.Time != "" {
		return asTimestamp(record.Time, formats)
	} else if record.Timestamp != "" {
		return asTimestamp(record.Timestamp, formats)
	}

	return 0, errMissingTimestamp
}

// asTimestamp will parse an ISO8601 string into an OpenTelemetry
// nanosecond timestamp. If the string cannot be parsed, it will
// return zero and the error.
func asTimestamp(s string, formats []string) (pcommon.Timestamp, error) {
	var err error
	var t time.Time
	// Try parsing with provided formats first
	for _, format := range formats {
		if t, err = time.Parse(format, s); err == nil {
			return pcommon.Timestamp(t.UnixNano()), nil
		}
	}

	// Fallback to ISO 8601 parsing if no format matches
	if t, err = iso8601.ParseString(s); err == nil {
		return pcommon.Timestamp(t.UnixNano()), nil
	}
	return 0, err
}

// asSeverity converts the Azure log level to equivalent
// OpenTelemetry severity numbers. If the log level is not
// valid, then the 'Unspecified' value is returned.
func asSeverity(number json.Number) plog.SeverityNumber {
	switch number.String() {
	case "Informational":
		return plog.SeverityNumberInfo
	case "Warning":
		return plog.SeverityNumberWarn
	case "Error":
		return plog.SeverityNumberError
	case "Critical":
		return plog.SeverityNumberFatal
	default:
		levelNumber, _ := number.Int64()
		if levelNumber > 0 {
			return plog.SeverityNumber(levelNumber)
		}

		return plog.SeverityNumberUnspecified
	}
}

func extractRawAttributes(log azureLogRecord) map[string]any {
	attrs := map[string]any{}

	attrs[azureCategory] = log.Category
	setIf(attrs, azureCorrelationID, log.CorrelationID)
	if log.DurationMs != nil {
		duration, err := strconv.ParseInt(log.DurationMs.String(), 10, 64)
		if err == nil {
			attrs[azureDuration] = duration
		}
	}
	if log.Identity != nil {
		attrs[azureIdentity] = *log.Identity
	}
	attrs[azureOperationName] = log.OperationName
	setIf(attrs, azureOperationVersion, log.OperationVersion)

	if log.Properties != nil {
		copyPropertiesAndApplySemanticConventions(log.Category, log.Properties, attrs)
	}

	setIf(attrs, azureResultDescription, log.ResultDescription)
	setIf(attrs, azureResultSignature, log.ResultSignature)
	setIf(attrs, azureResultType, log.ResultType)
	setIf(attrs, azureTenantID, log.TenantID)

	setIf(attrs, conventions.AttributeCloudRegion, log.Location)
	setIf(attrs, networkPeerAddress, log.CallerIPAddress)
	return attrs
}

func copyPropertiesAndApplySemanticConventions(category string, properties *any, attrs map[string]any) {
	if properties == nil {
		return
	}

	// TODO: check if this is a valid JSON string and parse it?
	switch p := (*properties).(type) {
	case map[string]any:
		attrsProps := map[string]any{}

		for k, v := range p {
			// Check for a complex conversion, e.g. AppServiceHTTPLogs.Protocol
			if complexConversion, ok := tryGetComplexConversion(category, k); ok {
				if complexConversion(k, v, attrs) {
					continue
				}
			}
			// Check for an equivalent Semantic Convention key
			if otelKey, ok := resourceLogKeyToSemConvKey(k, category); ok {
				attrs[otelKey] = normalizeValue(otelKey, v)
			} else {
				attrsProps[k] = v
			}
		}

		if len(attrsProps) > 0 {
			attrs[azureProperties] = attrsProps
		}
	default:
		// otherwise, just add the properties as-is
		attrs[azureProperties] = *properties
	}
}

func setIf(attrs map[string]any, key string, value *string) {
	if value != nil && *value != "" {
		attrs[key] = *value
	}
}
