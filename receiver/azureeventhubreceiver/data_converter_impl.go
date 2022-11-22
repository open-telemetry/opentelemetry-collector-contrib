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
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"

	"github.com/relvacode/iso8601"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

var notHexDigit = regexp.MustCompile(`[^a-fA-F0-9]+`)

// AzureRecords represents an array of Azure log records
// as exported via an Azure Event Hub
type azureRecords struct {
	Records []AzureLogRecord `json:"records"`
}

// AzureLogRecord represents a single Azure log following
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

// AsTimestamp will parse an ISO8601 string into an OpenTelemetry
// nanosecond timestamp. If the string cannot be parsed, it will
// return zero and the error.
func AsTimestamp(s string) (pcommon.Timestamp, error) {
	t, err := iso8601.ParseString(s)
	if err != nil {
		return 0, err
	}
	return pcommon.Timestamp(t.UnixNano()), nil
}

// AsSeverity converts the Azure log level to equivalent
// OpenTelemetry severity numbers. If the log level is not
// valid, then the 'Unspecified' value is returned.
func AsSeverity(s string) plog.SeverityNumber {
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

// SetIf will modify the given raw map by setting
// the key and value iff the value is not null and
// not the empty string.
func SetIf(attrs map[string]interface{}, key string, value *string) {
	if value != nil && *value != "" {
		attrs[key] = *value
	}
}

// TraceIDFromGUID will create a pcommon.TraceID from a
// hex representation of a GUID. All non-hex digits will
// be stripped from the string before the conversion. The
// function will return an empty TraceID for any error.
func TraceIDFromGUID(s *string) pcommon.TraceID {
	if s != nil {
		data, err := hex.DecodeString(notHexDigit.ReplaceAllString(*s, ""))
		if err == nil && len(data) == 16 {
			var binaryGUID [16]byte
			copy(binaryGUID[:], data[0:16])
			return binaryGUID
		}
	}
	return pcommon.NewTraceIDEmpty()
}

// JsonNumberToRaw converts a json.Number instance to either
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

// ReplaceJsonNumber will recursively scan through the
// given interface to find and replace json.Number
// instances with a raw int or double. This function
// returns the possibly mutated interface.
func ReplaceJsonNumber(i interface{}) interface{} {
	switch t := i.(type) {
	case map[string]interface{}:
		for k, v := range t {
			switch number := v.(type) {
			case json.Number:
				t[k] = JsonNumberToRaw(number)
			default:
				ReplaceJsonNumber(v)
			}
		}
	case []interface{}:
		for k, v := range t {
			switch number := v.(type) {
			case json.Number:
				t[k] = JsonNumberToRaw(number)
			default:
				ReplaceJsonNumber(v)
			}
		}
	}
	return i
}

// ExtractRawAttributes creates a raw attribute map and
// inserts attributes from the Azure log record. Optional
// attributes are only inserted if they are defined. The
// `azure.duration` value is only set if the value in the
// Azure log record can be parsed as an integer.
func extractRawAttributes(log AzureLogRecord) map[string]interface{} {
	var attrs = map[string]interface{}{}

	attrs["azure.resource.id"] = log.ResourceID
	SetIf(attrs, "azure.tenant.id", log.TenantID)
	attrs["azure.operation.name"] = log.OperationName
	SetIf(attrs, "azure.operation.version", log.OperationVersion)
	attrs["azure.category"] = log.Category
	SetIf(attrs, "azure.result.type", log.ResultType)
	SetIf(attrs, "azure.result.signature", log.ResultSignature)
	SetIf(attrs, "azure.result.description", log.ResultDescription)
	SetIf(attrs, "net.sock.peer.addr", log.CallerIPAddress)
	SetIf(attrs, "cloud.region", log.Location)
	attrs["cloud.provider"] = "azure"

	if log.DurationMs != nil {
		duration, err := strconv.ParseInt(*log.DurationMs, 10, 64)
		if err == nil {
			attrs["azure.duration"] = duration
		}
	}

	if log.Identity != nil {
		attrs["azure.identity"] = ReplaceJsonNumber(*log.Identity)
	}

	if log.Properties != nil {
		attrs["azure.properties"] = ReplaceJsonNumber(*log.Properties)
	}

	return attrs
}

// Transform takes a byte array containing a JSON-encoded
// payload with Azure log records and transforms it into
// an OpenTelemetry plog.Logs object. The data in the Azure
// log record appears as fields and attributes in the
// OpenTelemetry representation; the bodies of the
// OpenTelemetry log records are empty.
func transform(data []byte) (*plog.Logs, error) {

	l := plog.NewLogs()

	var azureLogs AzureRecords
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	err := decoder.Decode(&azureLogs)
	if err == nil {
		logRecords := l.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()

		for _, azureLog := range azureLogs.Records {
			nanos, err := AsTimestamp(azureLog.Time)
			if err == nil {
				lr := logRecords.AppendEmpty()

				lr.SetTimestamp(nanos)

				if azureLog.Level != nil {
					severity := AsSeverity(*azureLog.Level)
					lr.SetSeverityNumber(severity)
					lr.SetSeverityText(*azureLog.Level)
				}

				lr.SetTraceID(TraceIDFromGUID(azureLog.CorrelationID))

				lr.Attributes().FromRaw(ExtractRawAttributes(azureLog))
			}
		}
	}

	return &l, err
}
