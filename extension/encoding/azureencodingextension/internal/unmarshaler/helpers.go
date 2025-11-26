// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/relvacode/iso8601"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type RecordsBatchFormat string

// Supported wrapper formats of Azure Logs Records batch
const (
	FormatEventHub    RecordsBatchFormat = "eventhub"
	FormatBlobStorage RecordsBatchFormat = "blobstorage"
)

// JSON Path expressions that matches specific wrapper format
const (
	// As exported to Azure Event Hub, e.g. `{"records": [ {...}, {...} ]}`
	JSONPathEventHubLogRecords = "$.records[*]"
	// As exported to Azure Blob Storage, e.g. `[ {...}, {...} ]`
	JSONPathBlobStorageLogRecords = "$[*]"
)

// Commonly used attributes non-SemConv attributes across all telemetry signals
const (
	AttributeAzureCategory      = "azure.category"
	AttributeAzureOperationName = "azure.operation.name"
)

const originalSuffix = ".original"

// AsTimestamp tries to parse a string with timestamp into OpenTelemetry
// using provided list of formats layouts.
// If not formats provided or parsing using them failed - will use an ISO8601 parser.
// If the string cannot be parsed, it will return zero and the error.
func AsTimestamp(s string, formats ...string) (pcommon.Timestamp, error) {
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

// AttrPutStrIf is a helper function to set a string attribute
// only if the value is not empty
func AttrPutStrIf(attrs pcommon.Map, attrKey, attrValue string) {
	if attrValue != "" {
		attrs.PutStr(attrKey, attrValue)
	}
}

// AttrPutStrPtrIf is a helper function to set a string attribute
// only if the value exists and is not empty
func AttrPutStrPtrIf(attrs pcommon.Map, attrKey string, attrValue *string) {
	if attrValue != nil && *attrValue != "" {
		attrs.PutStr(attrKey, *attrValue)
	}
}

// AttrPutIntNumberIf is a helper function to set an int64 attribute with defined key,
// trying to parse it from json.Number value
// If parsing failed - no attribute will be set
func AttrPutIntNumberIf(attrs pcommon.Map, attrKey string, attrValue json.Number) {
	if i, err := attrValue.Int64(); err == nil {
		attrs.PutInt(attrKey, i)
	}
}

// AttrPutIntNumberPtrIf is a same function as AttrPutIntNumberIf but
// accepts a pointer to json.Number instead of value
func AttrPutIntNumberPtrIf(attrs pcommon.Map, attrKey string, attrValue *json.Number) {
	if attrValue != nil {
		AttrPutIntNumberIf(attrs, attrKey, *attrValue)
	}
}

// attrPutMap is a helper function to set a map attribute with defined key,
// trying to parse it from raw value
// If parsing failed - no attribute will be set
func AttrPutMapIf(attrs pcommon.Map, attrKey string, attrValue any) {
	if attrKey == "" || attrValue == nil {
		return
	}

	if err := attrs.PutEmpty(attrKey).FromRaw(attrValue); err != nil {
		// Failed to parse - put string representation of the attrValue
		attrs.Remove(attrKey)
		attrs.PutStr(attrKey+originalSuffix, fmt.Sprintf("%v", attrValue))
	}
}
