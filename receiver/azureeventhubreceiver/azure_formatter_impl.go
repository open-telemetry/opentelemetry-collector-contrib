// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azureeventhubreceiver // import "github.com/loomis-relativity/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"bytes"
	"encoding/json"
	"github.com/relvacode/iso8601"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"strconv"
)

const azureCategory = "azure.category"
const azureCorrelationID = "azure.correlation.id"
const azureDuration = "azure.duration"
const azureIdentity = "azure.identity"
const azureOperationName = "azure.operation.name"
const azureOperationVersion = "azure.operation.version"
const azureProperties = "azure.properties"
const azureResourceID = "azure.resource.id"
const azureResultType = "azure.result.type"
const azureResultSignature = "azure.result.signature"
const azureResultDescription = "azure.result.description"
const azureTenantID = "azure.tenant.id"

const cloudProvider = "cloud.provider"
const cloudRegion = "cloud.region"

const netSockPeerAddr = "net.sock.peer.addr"

// azureRecords represents an array of Azure log records
// as exported via an Azure Event Hub
type azureRecords struct {
	Records []azureLogRecord `json:"records"`
}

// azureLogRecord represents a single Azure log following
// the common schema:
// https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/resource-logs-schema
type azureLogRecord struct {
	Time              string       `json:"time"`
	ResourceID        string       `json:"resourceId"`
	TenantID          *string      `json:"tenantId"`
	OperationName     string       `json:"operationName"`
	OperationVersion  *string      `json:"operationVersion"`
	Category          string       `json:"category"`
	ResultType        *string      `json:"resultType"`
	ResultSignature   *string      `json:"resultSignature"`
	ResultDescription *string      `json:"resultDescription"`
	DurationMs        *string      `json:"durationMs"`
	CallerIPAddress   *string      `json:"callerIpAddress"`
	CorrelationID     *string      `json:"correlationId"`
	Identity          *interface{} `json:"identity"`
	Level             *string      `json:"Level"`
	Location          *string      `json:"location"`
	Properties        *interface{} `json:"properties"`
}

// asTimestamp will parse an ISO8601 string into an OpenTelemetry
// nanosecond timestamp. If the string cannot be parsed, it will
// return zero and the error.
func asTimestamp(s string) (pcommon.Timestamp, error) {
	t, err := iso8601.ParseString(s)
	if err != nil {
		return 0, err
	}
	return pcommon.Timestamp(t.UnixNano()), nil
}

// asSeverity converts the Azure log level to equivalent
// OpenTelemetry severity numbers. If the log level is not
// valid, then the 'Unspecified' value is returned.
func asSeverity(s string) plog.SeverityNumber {
	switch s {
	case "Informational":
		return plog.SeverityNumberInfo
	case "Warning":
		return plog.SeverityNumberWarn
	case "Error":
		return plog.SeverityNumberError
	case "Critical":
		return plog.SeverityNumberFatal
	default:
		return plog.SeverityNumberUnspecified
	}
}

// setIf will modify the given raw map by setting
// the key and value iff the value is not null and
// not the empty string.
func setIf(attrs map[string]interface{}, key string, value *string) {
	if value != nil && *value != "" {
		attrs[key] = *value
	}
}

// jsonNumberToRaw converts a json.Number instance to either
// a raw int64 or double value. If the value can be parsed
// as an integer, then the integer is returned, otherwise
// a double is returned. Nil may be returned if the
// json.Number instance is not valid.
func jsonNumberToRaw(n json.Number) interface{} {
	if i, err := n.Int64(); err == nil {
		return i
	}
	if f, err := n.Float64(); err == nil {
		return f
	}
	return nil
}

// replaceJsonNumber will recursively scan through the
// given interface to find and replace json.Number
// instances with a raw int or double. This function
// returns the possibly mutated interface.
func replaceJsonNumber(i interface{}) interface{} {
	switch t := i.(type) {
	case map[string]interface{}:
		for k, v := range t {
			switch number := v.(type) {
			case json.Number:
				t[k] = jsonNumberToRaw(number)
			default:
				replaceJsonNumber(v)
			}
		}
	case []interface{}:
		for k, v := range t {
			switch number := v.(type) {
			case json.Number:
				t[k] = jsonNumberToRaw(number)
			default:
				replaceJsonNumber(v)
			}
		}
	}
	return i
}

// extractRawAttributes creates a raw attribute map and
// inserts attributes from the Azure log record. Optional
// attributes are only inserted if they are defined. The
// azureDuration value is only set if the value in the
// Azure log record can be parsed as an integer.
func extractRawAttributes(log azureLogRecord) map[string]interface{} {
	var attrs = map[string]interface{}{}

	attrs[azureCategory] = log.Category
	setIf(attrs, azureCorrelationID, log.CorrelationID)
	if log.DurationMs != nil {
		duration, err := strconv.ParseInt(*log.DurationMs, 10, 64)
		if err == nil {
			attrs[azureDuration] = duration
		}
	}
	if log.Identity != nil {
		attrs[azureIdentity] = replaceJsonNumber(*log.Identity)
	}
	attrs[azureOperationName] = log.OperationName
	setIf(attrs, azureOperationVersion, log.OperationVersion)
	if log.Properties != nil {
		attrs[azureProperties] = replaceJsonNumber(*log.Properties)
	}
	setIf(attrs, azureResultDescription, log.ResultDescription)
	setIf(attrs, azureResultSignature, log.ResultSignature)
	setIf(attrs, azureResultType, log.ResultType)
	setIf(attrs, azureTenantID, log.TenantID)

	setIf(attrs, cloudRegion, log.Location)
	attrs[cloudProvider] = "azure"

	setIf(attrs, netSockPeerAddr, log.CallerIPAddress)

	return attrs
}

// transform takes a byte array containing a JSON-encoded
// payload with Azure log records and transforms it into
// an OpenTelemetry plog.Logs object. The data in the Azure
// log record appears as fields and attributes in the
// OpenTelemetry representation; the bodies of the
// OpenTelemetry log records are empty.
func transform(data []byte) (*plog.Logs, error) {

	l := plog.NewLogs()

	var azureLogs azureRecords
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	err := decoder.Decode(&azureLogs)
	resourceLogs := l.ResourceLogs().AppendEmpty()
	if err == nil {
		logRecords := resourceLogs.ScopeLogs().AppendEmpty().LogRecords()

		resourceID := ""
		for _, azureLog := range azureLogs.Records {
			resourceID = azureLog.ResourceID
			nanos, err := asTimestamp(azureLog.Time)
			if err == nil {
				lr := logRecords.AppendEmpty()

				lr.SetTimestamp(nanos)

				if azureLog.Level != nil {
					severity := asSeverity(*azureLog.Level)
					lr.SetSeverityNumber(severity)
					lr.SetSeverityText(*azureLog.Level)
				}

				//nolint:errcheck
				lr.Attributes().FromRaw(extractRawAttributes(azureLog))
			}
		}

		// The Azure resource ID will be pulled into a common resource attribute.
		// This implementation assumes that a single log message from Azure will
		// contain ONLY logs from a single resource.
		if resourceID != "" {
			resourceLogs.Resource().Attributes().PutStr(azureResourceID, resourceID)
		}
	}

	return &l, err
}
