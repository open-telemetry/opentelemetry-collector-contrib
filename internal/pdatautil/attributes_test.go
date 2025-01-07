// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestGetDimensionValue(t *testing.T) {
	resourceattribs := pcommon.NewMap()
	resourceattribs.PutStr("service.name", "mock-service-name")

	spanattris := pcommon.NewMap()
	spanattris.PutStr("span.name", "mock-span-name")

	otherattribs := pcommon.NewMap()
	otherattribs.PutStr("a", "b")
	otherattribs.PutStr("foo", "bar")

	defaultFoo := pcommon.NewValueStr("bar")

	tests := []struct {
		name             string
		dimension        Dimension
		attributes       []pcommon.Map
		wantDimensionVal string
	}{
		{
			name:             "success get dimension value",
			dimension:        Dimension{Name: "foo"},
			attributes:       []pcommon.Map{resourceattribs, spanattris, otherattribs},
			wantDimensionVal: "bar",
		},
		{
			name: "not found and get default dimension provided value",
			dimension: Dimension{
				Name:  "foo",
				Value: &defaultFoo,
			},
			attributes:       []pcommon.Map{resourceattribs, spanattris},
			wantDimensionVal: "bar",
		},
		{
			name: "not found and get default get empty value",
			dimension: Dimension{
				Name: "foo",
			},
			attributes:       []pcommon.Map{resourceattribs, spanattris},
			wantDimensionVal: "",
		},
	}

	for _, tc := range tests {
		val, ok := GetDimensionValue(tc.dimension, tc.attributes...)
		if ok {
			assert.Equal(t, tc.wantDimensionVal, val.AsString())
		}
	}
}
