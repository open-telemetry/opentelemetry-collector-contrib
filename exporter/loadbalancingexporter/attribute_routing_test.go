// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestBuildAttributeRoutingKeyValue_Primitives(t *testing.T) {
	tests := []struct {
		name  string
		value pcommon.Value
		want  string
	}{
		{name: "string", value: pcommon.NewValueStr("abc"), want: "k=abc|"},
		{name: "int", value: pcommon.NewValueInt(42), want: "k=42|"},
		{name: "double", value: pcommon.NewValueDouble(3.5), want: "k=3.5|"},
		{name: "bool", value: pcommon.NewValueBool(true), want: "k=true|"},
		{name: "empty", value: pcommon.NewValueEmpty(), want: "k=|"},
		{
			name: "bytes",
			value: func() pcommon.Value {
				v := pcommon.NewValueBytes()
				v.Bytes().FromRaw([]byte{0x01, 0xFF})
				return v
			}(),
			want: "k=Af8=|",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, buildAttributeRoutingKeyValue("k", tc.value))
		})
	}
}

func TestBuildAttributeRoutingKeyValue_MapAndSliceDeterministic(t *testing.T) {
	m1 := pcommon.NewValueMap()
	m1.Map().PutInt("b", 2)
	m1.Map().PutStr("a", "x")

	m2 := pcommon.NewValueMap()
	m2.Map().PutStr("a", "x")
	m2.Map().PutInt("b", 2)

	assert.Equal(t, "k={\"a\":\"x\",\"b\":2}|", buildAttributeRoutingKeyValue("k", m1))
	assert.Equal(t, buildAttributeRoutingKeyValue("k", m1), buildAttributeRoutingKeyValue("k", m2))

	s := pcommon.NewValueSlice()
	s.Slice().AppendEmpty().SetInt(1)
	s.Slice().AppendEmpty().SetStr("x")
	s.Slice().AppendEmpty().SetBool(false)

	assert.Equal(t, "k=[1,\"x\",false]|", buildAttributeRoutingKeyValue("k", s))
}
