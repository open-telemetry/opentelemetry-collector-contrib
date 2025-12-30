// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal"

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
)

func GetServiceName(resAttr pcommon.Map) string {
	if v, ok := resAttr.Get(string(conventions.ServiceNameKey)); ok {
		return v.AsString()
	}

	return ""
}

// AttributesToJSON converts a pcommon.Map to JSON string for StarRocks JSON column
func AttributesToJSON(attributes pcommon.Map) (string, error) {
	if attributes.Len() == 0 {
		return "{}", nil
	}

	attrMap := make(map[string]interface{})
	attributes.Range(func(k string, v pcommon.Value) bool {
		attrMap[k] = valueToInterface(v)
		return true
	})

	jsonBytes, err := json.Marshal(attrMap)
	if err != nil {
		return "{}", err
	}

	return string(jsonBytes), nil
}

func valueToInterface(v pcommon.Value) interface{} {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeInt:
		return v.Int()
	case pcommon.ValueTypeDouble:
		return v.Double()
	case pcommon.ValueTypeBool:
		return v.Bool()
	case pcommon.ValueTypeMap:
		m := make(map[string]interface{})
		v.Map().Range(func(k string, val pcommon.Value) bool {
			m[k] = valueToInterface(val)
			return true
		})
		return m
	case pcommon.ValueTypeSlice:
		slice := make([]interface{}, v.Slice().Len())
		for i := 0; i < v.Slice().Len(); i++ {
			slice[i] = valueToInterface(v.Slice().At(i))
		}
		return slice
	default:
		return v.AsString()
	}
}

// UniqueFlattenedAttributes converts a pcommon.Map into a slice of attributes. Paths are flattened and sorted.
func UniqueFlattenedAttributes(m pcommon.Map) []string {
	mLen := m.Len()
	if mLen == 0 {
		return nil
	}

	pathsSet := make(map[string]struct{}, mLen)
	paths := make([]string, 0, mLen)

	uniqueFlattenedAttributesNested("", &pathsSet, &paths, m)
	slices.Sort(paths)

	return paths
}

func uniqueFlattenedAttributesNested(pathPrefix string, pathsSet *map[string]struct{}, paths *[]string, m pcommon.Map) {
	m.Range(func(path string, v pcommon.Value) bool {
		if pathPrefix != "" {
			var b strings.Builder
			b.WriteString(pathPrefix)
			b.WriteRune('.')
			b.WriteString(path)
			path = b.String()
		}

		if v.Type() == pcommon.ValueTypeMap {
			uniqueFlattenedAttributesNested(path, pathsSet, paths, v.Map())
		} else if _, ok := (*pathsSet)[path]; !ok {
			(*pathsSet)[path] = struct{}{}
			*paths = append(*paths, path)
		}

		return true
	})
}

// FormatSQLValue formats a value for use in SQL INSERT statements.
// This function properly escapes strings and formats other types for StarRocks/MySQL compatibility.
// For JSON strings, it uses proper escaping that preserves JSON validity.
func FormatSQLValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}

	switch val := v.(type) {
	case string:
		// For MySQL/StarRocks string literals, we need to escape:
		// 1. Single quotes by doubling them ('')
		// 2. Backslashes by doubling them (\\)
		// 3. NULL bytes (\0)
		// 4. Control characters
		// However, for JSON strings, we need to be careful not to break JSON escape sequences.
		// The safest approach is to escape backslashes first, then single quotes.
		// This preserves JSON escape sequences like \" which become \\" in SQL, which is correct.
		escaped := strings.ReplaceAll(val, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "'", "''")
		// Also escape NULL bytes and other control characters that could break SQL
		escaped = strings.ReplaceAll(escaped, "\x00", "\\0")
		return "'" + escaped + "'"
	case bool:
		if val {
			return "1"
		}
		return "0"
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", val)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%g", val)
	case time.Time:
		// Format as MySQL/StarRocks datetime without timezone
		// StarRocks requires format: YYYY-MM-DD HH:MM:SS
		// Convert to UTC and format without timezone info
		utcTime := val.UTC()
		formatted := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
			utcTime.Year(), utcTime.Month(), utcTime.Day(),
			utcTime.Hour(), utcTime.Minute(), utcTime.Second())
		return "'" + formatted + "'"
	default:
		// Check if it's a *time.Time pointer
		if t, ok := val.(*time.Time); ok && t != nil {
			utcTime := t.UTC()
			formatted := fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d",
				utcTime.Year(), utcTime.Month(), utcTime.Day(),
				utcTime.Hour(), utcTime.Minute(), utcTime.Second())
			return "'" + formatted + "'"
		}
		// Check if it's a pcommon.Timestamp (int64 alias) or similar by using reflection
		// to get the underlying int64 value
		rv := reflect.ValueOf(val)
		if rv.Kind() == reflect.Int64 {
			return fmt.Sprintf("%d", rv.Int())
		}
		if rv.Kind() == reflect.Uint64 {
			return fmt.Sprintf("%d", rv.Uint())
		}
		// For unknown types, convert to string and escape
		str := fmt.Sprintf("%v", val)
		escaped := strings.ReplaceAll(str, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "'", "''")
		escaped = strings.ReplaceAll(escaped, "\x00", "\\0")
		return "'" + escaped + "'"
	}
}

// BuildValuesClause builds a VALUES clause from a list of values for SQL INSERT statements.
func BuildValuesClause(values []interface{}) string {
	if len(values) == 0 {
		return "()"
	}

	var b strings.Builder
	b.WriteString("(")
	for i, v := range values {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(FormatSQLValue(v))
	}
	b.WriteString(")")
	return b.String()
}
