// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package canonicaljson // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/canonicaljson"

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// valueToInterface converts pcommon.Value to Go interface{} for JSON serialization.
// Wraps primitive types in maps with typed keys per OTLP JSON encoding.
func valueToInterface(v pcommon.Value, depth, maxDepth int) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("value exceeds nesting depth limit (%d)", maxDepth)
	}
	switch v.Type() {
	case pcommon.ValueTypeStr:
		s := v.Str()
		if err := checkUTF8(s, "string value"); err != nil {
			return nil, err
		}
		return map[string]any{"stringValue": s}, nil
	case pcommon.ValueTypeInt:
		return map[string]any{"intValue": strconv.FormatInt(v.Int(), 10)}, nil
	case pcommon.ValueTypeDouble:
		return map[string]any{"doubleValue": v.Double()}, nil
	case pcommon.ValueTypeBool:
		return map[string]any{"boolValue": v.Bool()}, nil
	case pcommon.ValueTypeBytes:
		return map[string]any{"bytesValue": base64.StdEncoding.EncodeToString(v.Bytes().AsRaw())}, nil
	case pcommon.ValueTypeSlice:
		slice := make([]any, v.Slice().Len())
		for i := 0; i < v.Slice().Len(); i++ {
			val, err := valueToInterface(v.Slice().At(i), depth+1, maxDepth)
			if err != nil {
				return nil, err
			}
			slice[i] = val
		}
		return slice, nil
	case pcommon.ValueTypeMap:
		m := make(map[string]any)
		var mapErr error
		v.Map().Range(func(k string, val pcommon.Value) bool {
			if err := checkUTF8(k, "map key"); err != nil {
				mapErr = err
				return false
			}
			converted, err := valueToInterface(val, depth+1, maxDepth)
			if err != nil {
				mapErr = err
				return false
			}
			m[k] = converted
			return true
		})
		if mapErr != nil {
			return nil, mapErr
		}
		return m, nil
	case pcommon.ValueTypeEmpty:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported value type: %s", v.Type())
	}
}
