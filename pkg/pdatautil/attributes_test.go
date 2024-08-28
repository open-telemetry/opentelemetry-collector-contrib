// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil

import (
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"testing"
)

func TestGetDimensionValue(t *testing.T) {
	resource_attris := pcommon.NewMap()
	resource_attris.PutStr("service.name", "mock-service-name")

	span_attris := pcommon.NewMap()
	span_attris.PutStr("span.name", "mock-span-name")

	other_attris := pcommon.NewMap()
	other_attris.PutStr("a", "b")
	other_attris.PutStr("foo", "bar")

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
			attributes:       []pcommon.Map{resource_attris, span_attris, other_attris},
			wantDimensionVal: "bar",
		},
		{
			name: "not found and get default dimension provided value",
			dimension: Dimension{
				Name:  "foo",
				Value: &defaultFoo,
			},
			attributes:       []pcommon.Map{resource_attris, span_attris},
			wantDimensionVal: "bar",
		},
		{
			name: "not found and get default get empty value",
			dimension: Dimension{
				Name: "foo",
			},
			attributes:       []pcommon.Map{resource_attris, span_attris},
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
