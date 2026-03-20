// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/genaiadapterconnector/adapters/common"

import "encoding/json"

const MaxJSONDepth = 10

func ParseStr(val any) (string, bool) {
	if val == nil {
		return "", false
	}
	switch v := val.(type) {
	case string:
		if v == "null" {
			return "", false
		}
		return v, true
	default:
		if data, err := json.Marshal(v); err == nil {
			s := string(data)
			if s == "null" {
				return "", false
			}
			return s, true
		}
		return "", false
	}
}

func ParseInt(val any) (int64, bool) {
	switch v := val.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	}
	return 0, false
}

func ParseFloat(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	}
	return 0, false
}

func ParseJSON(val any, maxDepth int) any {
	return parseJSONRecurse(val, 0, maxDepth)
}

func parseJSONRecurse(val any, depth, maxDepth int) any {
	if depth >= maxDepth {
		return "..."
	}
	switch v := val.(type) {
	case map[string]any:
		result := make(map[string]any)
		for k, v := range v {
			result[k] = parseJSONRecurse(v, depth+1, maxDepth)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = parseJSONRecurse(item, depth+1, maxDepth)
		}
		return result
	default:
		return v
	}
}
