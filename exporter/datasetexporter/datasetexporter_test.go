// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter

import "go.opentelemetry.io/collector/pdata/pcommon"

func fillAttributes(attr pcommon.Map, allTypes bool, valueSuffix string) {
	attr.PutStr("string", "string"+valueSuffix)
	if allTypes {
		attr.PutDouble("double", 2.0)
		attr.PutBool("bool", true)
		attr.PutEmpty("empty")
		attr.PutInt("int", 3)
	}

	attr.PutEmptyMap("empty_map")
	mVal := attr.PutEmptyMap("map")
	mVal.PutStr("map_string", "map_string"+valueSuffix)
	mVal.PutEmpty("map_empty")
	mVal2 := mVal.PutEmptyMap("map_map")
	mVal2.PutStr("map_map_string", "map_map_string"+valueSuffix)

	attr.PutEmptySlice("empty_slice")
	sVal := attr.PutEmptySlice("slice")
	sVal.AppendEmpty()
	sVal.At(0).SetStr("slice_string" + valueSuffix)
}
