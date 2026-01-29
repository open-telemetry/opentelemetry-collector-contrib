// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/relvacode/iso8601"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
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
	// OpenTelemetry attribute name for Azure Resource Log Category,
	// mostly used in "logs" telemetry, but also can be present in "metrics" and "traces"
	AttributeAzureCategory = "azure.category"

	// OpenTelemetry attribute name for Azure Resource Operation Name
	AttributeAzureOperationName = "azure.operation.name"
)

const (
	// OpenTelemetry attribute name for Destination original address,
	// in case if the value could not be parsed into
	// `destination.address` and `destination.port` attributes
	attributeDestinationAddressOriginal = "destination.original_address"

	// OpenTelemetry attribute name for Client original address,
	// in case if the value could not be parsed into
	// `client.address` and `client.port` attributes
	attributeClientAddressOriginal = "client.original_address"

	// OpenTelemetry attribute name for Client original address,
	// in case if the value could not be parsed into
	// `server.address` and `server.port` attributes
	attributeServerAddressOriginal = "server.original_address"

	// OpenTelemetry attribute name for Network Peer original address,
	// in case if the value could not be parsed into
	// `network.peer.address` and `network.peer.port` attributes
	attributeNetworkPeerAddressOriginal = "network.peer.original_address"

	// OpenTelemetry attribute name for Network Local original address,
	// in case if the value could not be parsed into
	// `network.local.address` and `network.local.port` attributes
	attributeNetworkLocalAddressOriginal = "network.local.original_address"
)

const (
	originalSuffix = "_original"
	redactedStr    = "REDACTED"
	// When field value is set to "-" it's indicate that the data was unknown
	// or unavailable, or that the field was not applicable to this request
	unknownField = "-"
)

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
	if attrValue != "" && attrValue != unknownField {
		attrs.PutStr(attrKey, attrValue)
	}
}

// AttrPutStrPtrIf is a helper function to set a string attribute
// only if the value exists and is not empty
func AttrPutStrPtrIf(attrs pcommon.Map, attrKey string, attrValue *string) {
	if attrValue != nil && *attrValue != "" && *attrValue != unknownField {
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

// AttrPutIntNumberIf is a helper function to set an float64 attribute with defined key,
// trying to parse it from json.Number value
// If parsing failed - no attribute will be set
func AttrPutFloatNumberIf(attrs pcommon.Map, attrKey string, attrValue json.Number) {
	if i, err := attrValue.Float64(); err == nil {
		attrs.PutDouble(attrKey, i)
	}
}

// AttrPutFloatNumberPtrIf is a same function as AttrPutIntNumberPtrIf but
// accepts a pointer to json.Number instead of value
func AttrPutFloatNumberPtrIf(attrs pcommon.Map, attrKey string, attrValue *json.Number) {
	if attrValue != nil {
		AttrPutFloatNumberIf(attrs, attrKey, *attrValue)
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

// AttrPutURLParsed is a helper function that parses provided URI into set
// of OpenTelemetry SemConv attributes
// It will set `url.original` on any non-empty URI string and other SemConv
// attributes in case if provided URI can be parsed
// Puts maximum 8 attributes
func AttrPutURLParsed(attrs pcommon.Map, uri string) {
	if uri == "" {
		return
	}

	// Try parsing provided URI
	u, errURL := url.Parse(uri)
	if errURL != nil {
		// Put original URI only if parsing failed to avoid data duplication
		attrs.PutStr(string(conventions.URLOriginalKey), uri) // unstable SemConv
		return
	}
	// Mask credentials according to SemConv specs
	if u.User.String() != "" {
		u.User = url.UserPassword(redactedStr, redactedStr)
	}
	// Set url.full
	attrs.PutStr(string(conventions.URLFullKey), u.String())

	// Trying to parse port
	if port := u.Port(); port != "" {
		// We can safely ignore the parse error here because `url.Parse` will fail
		// in case of incorrect port, so it's parsable here 99%
		if intPort, err := strconv.ParseInt(port, 10, 64); err == nil {
			attrs.PutInt(string(conventions.URLPortKey), intPort) // unstable SemConv
		}
	}

	// Set other valuable `url.*` attributes according to SemConv
	AttrPutStrIf(attrs, string(conventions.URLSchemeKey), u.Scheme)
	AttrPutStrIf(attrs, string(conventions.URLDomainKey), u.Hostname()) // unstable SemConv
	AttrPutStrIf(attrs, string(conventions.URLPathKey), u.Path)
	AttrPutStrIf(attrs, string(conventions.URLQueryKey), u.RawQuery)
	AttrPutStrIf(attrs, string(conventions.URLFragmentKey), u.Fragment)
}

// AttrPutHostPortIf tries to parse provided `value` as "host:port" format and put result
// into respective `addrKey` for "host" and `portKey` for "port"
// If no port was detected in `value` - only `addrKey` will be set
// If value is not in "host:port" format or port is invalid - then
// `fallbackKey` attribute will be set with the original value
// Puts at most 2 attributes
func AttrPutHostPortIf(attrs pcommon.Map, addrKey, portKey, value string) {
	if value == "" {
		// Nothing to do here
		return
	}

	// Not a "host:port" format or only IPv6
	// put only "*.address" attribute
	if strings.LastIndexByte(value, ':') < 0 || (strings.Count(value, ":") > 1 && !strings.Contains(value, "]:")) {
		attrs.PutStr(addrKey, value)
		return
	}

	// Determine name of fallback attribute key
	var fallbackKey string
	switch addrKey {
	case string(conventions.DestinationAddressKey):
		fallbackKey = attributeDestinationAddressOriginal
	case string(conventions.ClientAddressKey):
		fallbackKey = attributeClientAddressOriginal
	case string(conventions.ServerAddressKey):
		fallbackKey = attributeServerAddressOriginal
	case string(conventions.NetworkPeerAddressKey):
		fallbackKey = attributeNetworkPeerAddressOriginal
	case string(conventions.NetworkLocalAddressKey):
		fallbackKey = attributeNetworkLocalAddressOriginal
	default:
		fallbackKey = addrKey + originalSuffix
	}

	// Try to parse host:port
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		attrs.PutStr(fallbackKey, value)
		return
	}
	// net.SplitHostPort actually does not validates if port is a valid number,
	// so will try to convert it and return error on failure
	nPort, err := strconv.ParseInt(port, 10, 64)
	if err != nil {
		attrs.PutStr(fallbackKey, value)
		return
	}
	attrs.PutStr(addrKey, host)
	attrs.PutInt(portKey, nPort)
}
