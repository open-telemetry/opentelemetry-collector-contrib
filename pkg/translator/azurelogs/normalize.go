// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"fmt"
	"strconv"
	"strings"
)

func normalizeValue(key string, val any) any {
	switch key {
	case
		"http.request.body.size",
		"http.request.size",
		"http.response.body.size",
		"http.response.size",
		"http.response.status_code",
		"server.port":
		return toInt(val)
	case "http.server.request.duration":
		return toFloat(val)
	case "network.protocol.name":
		return toLower(val)
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
	case float64:
		return v
	case float32:
		return float64(v)
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
