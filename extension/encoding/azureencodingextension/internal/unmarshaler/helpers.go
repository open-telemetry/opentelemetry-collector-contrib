// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode"

	"github.com/relvacode/iso8601"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type RecordsBatchFormat int

// Supported wrapper formats of Azure Logs Records batch
const (
	FormatUnknown RecordsBatchFormat = iota
	FormatObjectRecords
	FormatJSONArray
	FormatNDJSON
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

// DetectWrapperFormat tries to detect format based on provided bytes input
// At the moment we support only JSON Array, "records" and ND JSON formats,
// anything else is detected as unsupported format
func DetectWrapperFormat(input []byte) (RecordsBatchFormat, error) {
	iLen := len(input)
	// Not an error, just empty input
	if iLen == 0 {
		return FormatUnknown, nil
	}

	if input[0] == '[' {
		// That's seems to be JSON Array format, e.g. `[ {...}, {...} ]`
		return FormatJSONArray, nil
	}

	if input[0] != '{' {
		return FormatUnknown, errors.New("not a valid JSON or valid ND JSON")
	}

	maxBytes := min(iLen, 100)
	// We'll scan only first 100 bytes to avoid performance bottleneck here
	recordsIdx := bytes.Index(input[:maxBytes], []byte("\"records\""))
	if recordsIdx != -1 {
		// Make sure that it's a top-level field, i.e. before it we have only '{' not-whitespace character
		topSlice := bytes.TrimSpace(input[:recordsIdx])
		if len(topSlice) == 1 && topSlice[0] == '{' {
			// Make sure that "records" field is an array, i.e. next non-whitespace characters are ':' and '['
			nextChar := byte(':')
			for i := recordsIdx; i < maxBytes; i++ {
				if !unicode.IsSpace(rune(input[i])) && input[i] == nextChar {
					if nextChar == byte(':') {
						// Next character should be '['
						nextChar = byte('[')
					} else if nextChar == byte('[') {
						// That's seems to be JSON object format with "records" field, e.g. `{"records": [ {...}, {...} ]}`
						return FormatObjectRecords, nil
					}
				}
			}
		}
	}

	// Detect ND JSON
	idx := bytes.IndexByte(input, '\n')
	if idx > 1 && input[idx-1] == '}' {
		return FormatNDJSON, nil
	}

	// Everything else - is unsupported
	// Will include first bytes of input in error to simplify further debug
	return FormatUnknown, fmt.Errorf("unable to detect JSON format from input: %s", input[:maxBytes])
}

// AsTimestamp tries to parse a string with timestamp into OpenTelemetry
// using provided list of formats layouts.
// First is used ISO8601 parser - as it's the most common format for Azure telemetry data
// After that will be tested provided formats if any
// If the string cannot be parsed, it will return zero and the error.
func AsTimestamp(s string, formats ...string) (pcommon.Timestamp, error) {
	var err error
	var t time.Time

	// In most cases - Azure telemetry data has ISO8601 formatted timestamp
	// So, to optimize performance we'll check it first here
	if t, err = iso8601.ParseString(s); err == nil {
		return pcommon.Timestamp(t.UnixNano()), nil
	}

	// Try parsing with provided formats first
	for _, format := range formats {
		if t, err = time.Parse(format, s); err == nil {
			return pcommon.Timestamp(t.UnixNano()), nil
		}
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
func AttrPutMapIf(attrs pcommon.Map, attrKey string, attrValue map[string]any) {
	if attrKey == "" || attrValue == nil {
		return
	}

	if err := attrs.PutEmpty(attrKey).FromRaw(attrValue); err != nil {
		// Failed to parse - put string representation of the attrValue
		attrs.Remove(attrKey)
		attrs.PutStr(attrKey+originalSuffix, fmt.Sprintf("%v", attrValue))
	}
}
