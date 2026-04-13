// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/attribute"
)

func setMapAttribute(dest pcommon.Map, attr attribute.KeyValue) {
	key := string(attr.Key)

	switch attr.Value.Type() {
	case attribute.BOOL:
		dest.PutBool(key, attr.Value.AsBool())
	case attribute.INT64:
		dest.PutInt(key, attr.Value.AsInt64())
	case attribute.FLOAT64:
		dest.PutDouble(key, attr.Value.AsFloat64())
	case attribute.STRING:
		dest.PutStr(key, attr.Value.AsString())
	case attribute.BOOLSLICE:
		slice := dest.PutEmptySlice(key)
		for _, value := range attr.Value.AsBoolSlice() {
			slice.AppendEmpty().SetBool(value)
		}
	case attribute.INT64SLICE:
		slice := dest.PutEmptySlice(key)
		for _, value := range attr.Value.AsInt64Slice() {
			slice.AppendEmpty().SetInt(value)
		}
	case attribute.FLOAT64SLICE:
		slice := dest.PutEmptySlice(key)
		for _, value := range attr.Value.AsFloat64Slice() {
			slice.AppendEmpty().SetDouble(value)
		}
	case attribute.STRINGSLICE:
		slice := dest.PutEmptySlice(key)
		for _, value := range attr.Value.AsStringSlice() {
			slice.AppendEmpty().SetStr(value)
		}
	}
}

func setProfileAttributeValue(dest pcommon.Value, value attribute.Value) {
	switch value.Type() {
	case attribute.BOOL:
		dest.SetBool(value.AsBool())
	case attribute.INT64:
		dest.SetInt(value.AsInt64())
	case attribute.FLOAT64:
		dest.SetDouble(value.AsFloat64())
	case attribute.STRING:
		dest.SetStr(value.AsString())
	case attribute.BOOLSLICE:
		slice := dest.SetEmptySlice()
		for _, item := range value.AsBoolSlice() {
			slice.AppendEmpty().SetBool(item)
		}
	case attribute.INT64SLICE:
		slice := dest.SetEmptySlice()
		for _, item := range value.AsInt64Slice() {
			slice.AppendEmpty().SetInt(item)
		}
	case attribute.FLOAT64SLICE:
		slice := dest.SetEmptySlice()
		for _, item := range value.AsFloat64Slice() {
			slice.AppendEmpty().SetDouble(item)
		}
	case attribute.STRINGSLICE:
		slice := dest.SetEmptySlice()
		for _, item := range value.AsStringSlice() {
			slice.AppendEmpty().SetStr(item)
		}
	}
}
