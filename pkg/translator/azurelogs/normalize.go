// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"fmt"
	"strconv"
	"strings"
)

type valueNormalizer func(any) any

var normalizers = map[string]valueNormalizer{
	"http.request.body.size":       toInt,
	"http.request.size":            toInt,
	"http.response.body.size":      toInt,
	"http.response.size":           toInt,
	"http.response.status_code":    toInt,
	"http.server.request.duration": toFloat,
	"network.protocol.name":        toLower,
	"server.port":                  toInt,
}

func normalizeValue(key string, val any) any {
	if f, exists := normalizers[key]; exists {
		return f(val)
	}
	return val
}

func toLower(value any) any {
	switch v := value.(type) {
	case string:
		return strings.ToLower(v)
	default:
		return strings.ToLower(fmt.Sprint(value))
	}
}

func toFloat(value any) any {
	switch v := value.(type) {
	case float32:
		return float64(v)
	case float64:
		return v
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return f
		}
	}
	return value
}

func toInt(value any) any {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(int(v))
	case int64:
		return value.(int64)
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return i
		}
	}
	return value
}
