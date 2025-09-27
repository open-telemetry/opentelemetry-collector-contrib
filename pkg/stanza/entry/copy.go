// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
)

// copyValue will deep copy a value based on its type.
func copyValue(v any) any {
	switch value := v.(type) {
	case string, int, bool, byte, nil:
		return value
	case map[string]string:
		return maps.Clone(value)
	case map[string]any:
		return copyAnyMap(value)
	case []string:
		return slices.Clone(value)
	case []byte:
		return bytes.Clone(value)
	case []int:
		return slices.Clone(value)
	case []any:
		return copyAnySlice(value)
	default:
		return copyUnknown(value)
	}
}

// copyAnyMap will deep copy a map of interfaces.
func copyAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	mapCopy := make(map[string]any, len(m))
	for k, v := range m {
		mapCopy[k] = copyValue(v)
	}
	return mapCopy
}

// copyAnySlice will deep copy an array of interfaces.
func copyAnySlice(a []any) []any {
	if a == nil {
		return nil
	}
	arrayCopy := make([]any, 0, len(a))
	for _, v := range a {
		arrayCopy = append(arrayCopy, copyValue(v))
	}
	return arrayCopy
}

// copyUnknown will copy an unknown value using json encoding.
// If this process fails, the result will be an empty interface.
func copyUnknown(value any) any {
	var result any
	b, _ := json.Marshal(value)
	_ = json.Unmarshal(b, &result)
	return result
}
