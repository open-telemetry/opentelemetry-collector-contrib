// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package shared // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"

import (
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// strToInt converts a string representation of a number into a 64-bit integer.
// The string must contain a valid base-10 integer. If parsing fails, the function
// returns -1 and an error describing the failure.
func strToInt(numberStr string) (int64, error) {
	num, err := strconv.ParseInt(numberStr, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to convert string %q to int64", numberStr)
	}
	return num, nil
}

// AddStrAsInt parses a string value into an integer and adds it to the attributes map.
// If the input string is empty, the function does nothing and returns nil.
// Returns an error if the string cannot be parsed as an integer.
func AddStrAsInt(field, value string, attributes pcommon.Map) error {
	if value == "" {
		return nil
	}
	n, err := strToInt(value)
	if err != nil {
		return err
	}
	attributes.PutInt(field, n)
	return nil
}

// PutStr places value in the attributes map, if not empty
func PutStr(field, value string, attributes pcommon.Map) {
	if value != "" {
		attributes.PutStr(field, value)
	}
}

// PutInt places value in the attributes map, if not nil
func PutInt(field string, value *int64, attributes pcommon.Map) {
	if value != nil {
		attributes.PutInt(field, *value)
	}
}

// PutBool places the value in the attributes map, if not nil
func PutBool(field string, value *bool, attributes pcommon.Map) {
	if value != nil {
		attributes.PutBool(field, *value)
	}
}
