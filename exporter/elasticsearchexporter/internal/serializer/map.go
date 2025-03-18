// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"

import (
	"bytes"
	"encoding/hex"

	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func Map(m pcommon.Map, buf *bytes.Buffer) {
	v := json.NewVisitor(buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	writeMap(v, m, false)
}

func writeMap(v *json.Visitor, m pcommon.Map, stringifyMapValues bool) {
	_ = v.OnObjectStart(-1, structform.AnyType)
	for k, val := range m.All() {
		_ = v.OnKey(k)
		WriteValue(v, val, stringifyMapValues)
	}
	_ = v.OnObjectFinished()
}

func WriteValue(v *json.Visitor, val pcommon.Value, stringifyMaps bool) {
	switch val.Type() {
	case pcommon.ValueTypeEmpty:
		_ = v.OnNil()
	case pcommon.ValueTypeStr:
		_ = v.OnString(val.Str())
	case pcommon.ValueTypeBool:
		_ = v.OnBool(val.Bool())
	case pcommon.ValueTypeDouble:
		_ = v.OnFloat64(val.Double())
	case pcommon.ValueTypeInt:
		_ = v.OnInt64(val.Int())
	case pcommon.ValueTypeBytes:
		_ = v.OnString(hex.EncodeToString(val.Bytes().AsRaw()))
	case pcommon.ValueTypeMap:
		if stringifyMaps {
			_ = v.OnString(val.AsString())
		} else {
			writeMap(v, val.Map(), false)
		}
	case pcommon.ValueTypeSlice:
		_ = v.OnArrayStart(-1, structform.AnyType)
		slice := val.Slice()
		for i := 0; i < slice.Len(); i++ {
			WriteValue(v, slice.At(i), stringifyMaps)
		}
		_ = v.OnArrayFinished()
	}
}
