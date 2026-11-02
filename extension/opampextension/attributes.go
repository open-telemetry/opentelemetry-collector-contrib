// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opampextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension"

import (
	"slices"

	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func pcommonMapToKeyValues(attrs pcommon.Map) []*protobufs.KeyValue {
	if !isValidPcommonMap(attrs) || attrs.Len() == 0 {
		return nil
	}

	keys := make([]string, 0, attrs.Len())
	attrs.Range(func(k string, _ pcommon.Value) bool {
		keys = append(keys, k)
		return true
	})
	slices.Sort(keys)

	kvs := make([]*protobufs.KeyValue, 0, len(keys))
	for _, k := range keys {
		v, _ := attrs.Get(k)
		if av := pcommonValueToAnyValue(v); av != nil {
			kvs = append(kvs, &protobufs.KeyValue{
				Key:   k,
				Value: av,
			})
		}
	}

	return kvs
}

func isValidPcommonMap(attrs pcommon.Map) bool {
	valid := false
	func() {
		defer func() {
			if recover() != nil {
				valid = false
			}
		}()
		_ = attrs.Len()
		valid = true
	}()
	return valid
}

func pcommonValueToAnyValue(v pcommon.Value) *protobufs.AnyValue {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		return nil
	case pcommon.ValueTypeStr:
		return &protobufs.AnyValue{
			Value: &protobufs.AnyValue_StringValue{StringValue: v.Str()},
		}
	case pcommon.ValueTypeInt:
		return &protobufs.AnyValue{
			Value: &protobufs.AnyValue_IntValue{IntValue: v.Int()},
		}
	case pcommon.ValueTypeDouble:
		return &protobufs.AnyValue{
			Value: &protobufs.AnyValue_DoubleValue{DoubleValue: v.Double()},
		}
	case pcommon.ValueTypeBool:
		return &protobufs.AnyValue{
			Value: &protobufs.AnyValue_BoolValue{BoolValue: v.Bool()},
		}
	case pcommon.ValueTypeBytes:
		return &protobufs.AnyValue{
			Value: &protobufs.AnyValue_BytesValue{BytesValue: v.Bytes().AsRaw()},
		}
	case pcommon.ValueTypeSlice:
		slice := v.Slice()
		values := make([]*protobufs.AnyValue, 0, slice.Len())
		for i := 0; i < slice.Len(); i++ {
			if av := pcommonValueToAnyValue(slice.At(i)); av != nil {
				values = append(values, av)
			}
		}
		return &protobufs.AnyValue{
			Value: &protobufs.AnyValue_ArrayValue{
				ArrayValue: &protobufs.ArrayValue{Values: values},
			},
		}
	case pcommon.ValueTypeMap:
		return &protobufs.AnyValue{
			Value: &protobufs.AnyValue_KvlistValue{
				KvlistValue: &protobufs.KeyValueList{
					Values: pcommonMapToKeyValues(v.Map()),
				},
			},
		}
	default:
		return nil
	}
}
