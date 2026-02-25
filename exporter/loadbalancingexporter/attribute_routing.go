// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"encoding/base64"
	"sort"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// buildAttributeRoutingKey encodes a missing attribute as "name=|".
func buildAttributeRoutingKey(attr string) string {
	return attr + "=|"
}

// buildAttributeRoutingKeyStrValue encodes a single string attribute key/value pair as
// "name=value|".
func buildAttributeRoutingKeyStrValue(attr, value string) string {
	return attr + "=" + value + "|"
}

// buildAttributeRoutingKeyValue encodes a single attribute key/value pair as
// "name=value|".
func buildAttributeRoutingKeyValue(attr string, value pcommon.Value) string {
	return attr + "=" + attributeRoutingValueToString(value) + "|"
}

func attributeRoutingValueToString(value pcommon.Value) string {
	switch value.Type() {
	case pcommon.ValueTypeStr:
		return value.Str()
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(value.Int(), 10)
	case pcommon.ValueTypeDouble:
		return strconv.FormatFloat(value.Double(), 'g', -1, 64)
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(value.Bool())
	case pcommon.ValueTypeBytes:
		return base64.StdEncoding.EncodeToString(value.Bytes().AsRaw())
	case pcommon.ValueTypeMap:
		return attributeRoutingMapToString(value.Map())
	case pcommon.ValueTypeSlice:
		return attributeRoutingSliceToString(value.Slice())
	case pcommon.ValueTypeEmpty:
		return ""
	default:
		return ""
	}
}

func attributeRoutingMapToString(m pcommon.Map) string {
	keys := make([]string, 0, m.Len())
	for k := range m.All() {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte(':')
		v, _ := m.Get(k)
		b.WriteString(attributeRoutingValueToString(v))
	}
	b.WriteString("}")
	return b.String()
}

func attributeRoutingSliceToString(s pcommon.Slice) string {
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < s.Len(); i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(attributeRoutingValueToString(s.At(i)))
	}
	b.WriteString("]")
	return b.String()
}
