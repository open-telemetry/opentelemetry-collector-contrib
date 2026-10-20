// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlcommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"

import (
	"encoding/hex"
	"fmt"

	"github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlerror"
)

// ToString converts val to its string representation. The returned bool is false if val was nil.
// An error is returned if val cannot be converted.
func ToString(val any) (string, bool, error) {
	if val == nil {
		return "", false, nil
	}
	var result string
	switch v := val.(type) {
	case string:
		result = v
	case []byte:
		result = hex.EncodeToString(v)
	case pcommon.Map:
		resultBytes, err := json.Marshal(v.AsRaw())
		if err != nil {
			return "", false, err
		}
		result = string(resultBytes)
	case pcommon.Slice:
		resultBytes, err := json.Marshal(v.AsRaw())
		if err != nil {
			return "", false, err
		}
		result = string(resultBytes)
	case pcommon.Value:
		result = v.AsString()
	default:
		resultBytes, err := json.Marshal(v)
		if err != nil {
			return "", false, ottlerror.TypeError(fmt.Sprintf("unsupported type: %T", v))
		}
		result = string(resultBytes)
	}
	return result, true, nil
}

// IsMapOrList reports whether val is an OTTL map or list: a pcommon.Map, pcommon.Slice, a
// pcommon.Value holding either, or the raw Go equivalents (map[string]any, []any, []string,
// []int64, []float64 and []bool). A []byte is not considered a list.
func IsMapOrList(val any) bool {
	switch v := val.(type) {
	case pcommon.Map, pcommon.Slice, map[string]any, []any, []string, []int64, []float64, []bool:
		return true
	case pcommon.Value:
		return v.Type() == pcommon.ValueTypeMap || v.Type() == pcommon.ValueTypeSlice
	default:
		return false
	}
}
