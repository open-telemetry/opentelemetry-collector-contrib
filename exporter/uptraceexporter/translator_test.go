// Copyright OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package uptraceexporter

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func fillAttributeMap(m pdata.AttributeMap) {
	m.InsertString("string", "bar")
	m.InsertBool("bool", true)
	m.InsertInt("int", 123)
	m.InsertDouble("double", 0.123)
	m.InsertNull("null")
	m.Insert("map", pdata.NewAttributeValueMap())

	arrVal := pdata.NewAttributeValueArray()
	arr := arrVal.ArrayVal()
	arr.AppendEmpty().SetBoolVal(true)
	arr.AppendEmpty().SetBoolVal(false)
	m.Insert("array", arrVal)
}

func TestMapLabelValue(t *testing.T) {
	m := pdata.NewAttributeMap()
	fillAttributeMap(m)

	value, ok := mapLabelValue(m)
	require.True(t, ok)
	require.Equal(t, attribute.STRING, value.Type())
	require.Equal(t, `{"array":[true,false],"bool":true,"double":0.123,"int":123,"map":{},"null":null,"string":"bar"}`, value.AsString())

	e := &traceExporter{
		logger: zap.NewNop(),
	}
	kvSlice := e.keyValueSlice(m)
	require.Len(t, kvSlice, 6)
}

func TestInvalidMapLabelValue(t *testing.T) {
	m := pdata.NewAttributeMap()
	m.InsertDouble("double", math.NaN())

	_, ok := mapLabelValue(m)
	require.False(t, ok)
}

func TestEmptyArrayLabelValue(t *testing.T) {
	_, ok := arrayLabelValue(pdata.NewAnyValueArray())
	require.False(t, ok)
}

func TestArrayLabelValue(t *testing.T) {
	arr := pdata.NewAnyValueArray()
	arr.AppendEmpty()
	_, ok := arrayLabelValue(arr)
	require.False(t, ok)

	mapVal := pdata.NewAttributeValueMap()
	fillAttributeMap(mapVal.MapVal())

	arr = pdata.NewAnyValueArray()
	mapVal.CopyTo(arr.AppendEmpty())
	value, ok := arrayLabelValue(arr)
	require.True(t, ok)
	require.Equal(t, attribute.STRING, value.Type())

	type Test struct {
		val pdata.AttributeValue
	}

	tests := []Test{
		{val: pdata.NewAttributeValueBool(true)},
		{val: pdata.NewAttributeValueString("hello")},
		{val: pdata.NewAttributeValueInt(123)},
		{val: pdata.NewAttributeValueDouble(0.123)},
	}
	for _, test := range tests {
		arr := pdata.NewAnyValueArray()
		test.val.CopyTo(arr.AppendEmpty())

		value, ok := arrayLabelValue(arr)
		require.True(t, ok)
		require.Equal(t, attribute.ARRAY, value.Type())

		arr.AppendEmpty()
		_, ok = arrayLabelValue(arr)
		require.False(t, ok)
	}
}

func TestSpanKind(t *testing.T) {
	type Test struct {
		in  pdata.SpanKind
		out string
	}

	tests := []Test{
		{pdata.SpanKindClient, "client"},
		{pdata.SpanKindServer, "server"},
		{pdata.SpanKindProducer, "producer"},
		{pdata.SpanKindConsumer, "consumer"},
		{pdata.SpanKindInternal, "internal"},
	}

	for _, test := range tests {
		out := spanKind(test.in)
		require.Equal(t, test.out, out)
	}
}

func TestStatusCode(t *testing.T) {
	type Test struct {
		in  pdata.StatusCode
		out string
	}

	tests := []Test{
		{pdata.StatusCodeOk, "ok"},
		{pdata.StatusCodeError, "error"},
		{pdata.StatusCodeUnset, "unset"},
	}

	for _, test := range tests {
		out := statusCode(test.in)
		require.Equal(t, test.out, out)
	}
}
