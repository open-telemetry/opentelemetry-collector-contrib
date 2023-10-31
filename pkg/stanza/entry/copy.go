// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"

import "encoding/json"

// copyValue will deep copy a value based on its type.
func copyValue(v interface{}) interface{} {
	switch value := v.(type) {
	case string, int, bool, byte, nil:
		return value
	case map[string]string:
		return copyStringMap(value)
	case map[string]interface{}:
		return copyInterfaceMap(value)
	case []string:
		return copyStringArray(value)
	case []byte:
		return copyByteArray(value)
	case []int:
		return copyIntArray(value)
	case []interface{}:
		return copyInterfaceArray(value)
	default:
		return copyUnknown(value)
	}
}

// copyStringMap will deep copy a map of strings.
func copyStringMap(m map[string]string) map[string]string {
	mapCopy := make(map[string]string)
	for k, v := range m {
		mapCopy[k] = v
	}
	return mapCopy
}

// copyInterfaceMap will deep copy a map of interfaces.
func copyInterfaceMap(m map[string]interface{}) map[string]interface{} {
	mapCopy := make(map[string]interface{})
	for k, v := range m {
		mapCopy[k] = copyValue(v)
	}
	return mapCopy
}

// copyStringArray will deep copy an array of strings.
func copyStringArray(a []string) []string {
	arrayCopy := make([]string, len(a))
	copy(arrayCopy, a)
	return arrayCopy
}

// copyByteArray will deep copy an array of bytes.
func copyByteArray(a []byte) []byte {
	arrayCopy := make([]byte, len(a))
	copy(arrayCopy, a)
	return arrayCopy
}

// copyIntArray will deep copy an array of ints.
func copyIntArray(a []int) []int {
	arrayCopy := make([]int, len(a))
	copy(arrayCopy, a)
	return arrayCopy
}

// copyInterfaceArray will deep copy an array of interfaces.
func copyInterfaceArray(a []interface{}) []interface{} {
	arrayCopy := make([]interface{}, 0, len(a))
	for _, v := range a {
		arrayCopy = append(arrayCopy, copyValue(v))
	}
	return arrayCopy
}

// copyUnknown will copy an unknown value using json encoding.
// If this process fails, the result will be an empty interface.
func copyUnknown(value interface{}) interface{} {
	var result interface{}
	b, _ := json.Marshal(value)
	_ = json.Unmarshal(b, &result)
	return result
}
