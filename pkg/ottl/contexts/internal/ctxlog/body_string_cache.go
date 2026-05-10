// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ctxlog"

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type bodyStringCacheCarrier interface {
	GetCachedBodyString() (string, bool)
	SetCachedBodyString(string)
	InvalidateCachedBodyString()
}

func CacheBodyStringIfNeeded[K Context](tCtx K) {
	body := tCtx.GetLogRecord().Body()
	if !bodySupportsCachedString(body) {
		return
	}

	cacheCtx, ok := any(tCtx).(bodyStringCacheCarrier)
	if !ok {
		return
	}
	if _, ok := cacheCtx.GetCachedBodyString(); ok {
		return
	}

	cacheCtx.SetCachedBodyString(bodyAsStringOptimized(body))
}

func invalidateCachedBodyString[K any](tCtx K) {
	if cacheCtx, ok := any(tCtx).(bodyStringCacheCarrier); ok {
		cacheCtx.InvalidateCachedBodyString()
	}
}

func getCachedBodyString[K any](tCtx K) (string, bool) {
	cacheCtx, ok := any(tCtx).(bodyStringCacheCarrier)
	if !ok {
		return "", false
	}
	return cacheCtx.GetCachedBodyString()
}

func cacheBodyString[K any](tCtx K, bodyString string) bool {
	cacheCtx, ok := any(tCtx).(bodyStringCacheCarrier)
	if !ok {
		return false
	}
	cacheCtx.SetCachedBodyString(bodyString)
	return true
}

func bodySupportsCachedString(body pcommon.Value) bool {
	switch body.Type() {
	case pcommon.ValueTypeMap, pcommon.ValueTypeSlice:
		return true
	default:
		return false
	}
}

func bodyAsStringOptimized(v pcommon.Value) string {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		return ""
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(v.Bool())
	case pcommon.ValueTypeDouble:
		return float64AsJSONString(v.Double())
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(v.Int(), 10)
	case pcommon.ValueTypeMap:
		if s, ok := mapToJSONString(v.Map()); ok {
			return s
		}
		return v.AsString()
	case pcommon.ValueTypeBytes:
		return base64.StdEncoding.EncodeToString(v.Bytes().AsRaw())
	case pcommon.ValueTypeSlice:
		if s, ok := sliceToJSONString(v.Slice()); ok {
			return s
		}
		return v.AsString()
	default:
		return fmt.Sprintf("<Unknown OpenTelemetry attribute value type %q>", v.Type())
	}
}

func mapToJSONString(m pcommon.Map) (string, bool) {
	keys := make([]string, 0, m.Len())
	m.Range(func(k string, _ pcommon.Value) bool {
		keys = append(keys, k)
		return true
	})
	sort.Strings(keys)

	var b strings.Builder
	b.Grow(m.Len() * 16)
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(mustMarshalJSONString(k))
		b.WriteByte(':')
		v, _ := m.Get(k)
		if !appendJSONValue(&b, v) {
			return "", false
		}
	}
	b.WriteByte('}')
	return b.String(), true
}

func sliceToJSONString(s pcommon.Slice) (string, bool) {
	var b strings.Builder
	b.Grow(s.Len() * 12)
	b.WriteByte('[')
	for i := 0; i < s.Len(); i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		if !appendJSONValue(&b, s.At(i)) {
			return "", false
		}
	}
	b.WriteByte(']')
	return b.String(), true
}

func appendJSONValue(b *strings.Builder, v pcommon.Value) bool {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		b.WriteString("null")
	case pcommon.ValueTypeStr:
		b.WriteString(mustMarshalJSONString(v.Str()))
	case pcommon.ValueTypeBool:
		b.WriteString(strconv.FormatBool(v.Bool()))
	case pcommon.ValueTypeDouble:
		if math.IsInf(v.Double(), 0) || math.IsNaN(v.Double()) {
			return false
		}
		b.WriteString(float64AsJSONString(v.Double()))
	case pcommon.ValueTypeInt:
		b.WriteString(strconv.FormatInt(v.Int(), 10))
	case pcommon.ValueTypeMap:
		s, ok := mapToJSONString(v.Map())
		if !ok {
			return false
		}
		b.WriteString(s)
	case pcommon.ValueTypeSlice:
		s, ok := sliceToJSONString(v.Slice())
		if !ok {
			return false
		}
		b.WriteString(s)
	case pcommon.ValueTypeBytes:
		b.WriteString(mustMarshalJSONBytes(v.Bytes().AsRaw()))
	default:
		b.WriteString(mustMarshalJSONString(fmt.Sprintf("%v", v)))
	}
	return true
}

func mustMarshalJSONString(s string) string {
	encoded, err := json.Marshal(s)
	if err != nil {
		return "\"\""
	}
	return string(encoded)
}

func mustMarshalJSONBytes(bs []byte) string {
	encoded, err := json.Marshal(bs)
	if err != nil {
		return "\"\""
	}
	return string(encoded)
}

// Same float formatting behavior as pdata Value.AsString().
func float64AsJSONString(f float64) string {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return "json: unsupported value: " + strconv.FormatFloat(f, 'g', -1, 64)
	}

	scratch := [64]byte{}
	b := scratch[:0]
	fmtCode := byte('f')
	abs := math.Abs(f)
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		fmtCode = 'e'
	}
	b = strconv.AppendFloat(b, f, fmtCode, -1, 64)
	if fmtCode == 'e' {
		n := len(b)
		if n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
	}
	return string(b)
}
