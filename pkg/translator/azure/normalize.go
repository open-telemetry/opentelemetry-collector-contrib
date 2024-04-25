package azure

import (
	"fmt"
	"strconv"
	"strings"
)

const maxInt32 = int64(int32(^uint32(0) >> 1))

type valueNormalizer func(any) any

var normalizers = map[string]valueNormalizer{
	"http.request.size":            toInt,
	"http.response.size":           toInt,
	"http.server.request.duration": toFloat,
	"network.protocol.name":        toLower,
	"http.response.status_code":    toInt,
}

func normalizeValue(key string, val any) any {
	if f, exists := normalizers[key]; exists {
		return f(val)
	}
	return val
}

func toLower(value any) any {
	switch value.(type) {
	case string:
		return strings.ToLower(value.(string))
	default:
		return strings.ToLower(fmt.Sprint(value))
	}
}

func toFloat(value any) any {
	switch value.(type) {
	case float32:
		return float64(value.(float32))
	case float64:
		return value.(float64)
	case int:
		return float64(value.(int))
	case int32:
		return float64(value.(int32))
	case int64:
		return float64(value.(int64))
	case string:
		f, err := strconv.ParseFloat(value.(string), 64)
		if err == nil {
			return f
		}
	}
	return value
}

func toInt(value any) any {
	switch value.(type) {
	case int:
		return int64(value.(int))
	case int32:
		return int64(int(value.(int32)))
	case int64:
		return value.(int64)
	case string:
		i, err := strconv.ParseInt(value.(string), 10, 64)
		if err == nil {
			return i
		}
	}
	return value
}
