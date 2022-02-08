// Copyright The OpenTelemetry Authors
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

package lokiexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func exampleLog() (pdata.LogRecord, pdata.Resource) {

	buffer := pdata.NewLogRecord()
	buffer.Body().SetStringVal("Example log")
	buffer.SetSeverityText("error")
	buffer.Attributes().Insert("attr1", pdata.NewAttributeValueString("1"))
	buffer.Attributes().Insert("attr2", pdata.NewAttributeValueString("2"))
	buffer.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
	buffer.SetSpanID(pdata.NewSpanID([8]byte{5, 6, 7, 8}))

	resource := pdata.NewResource()
	resource.Attributes().Insert("host.name", pdata.NewAttributeValueString("something"))

	return buffer, resource
}

func TestConvertWithStringBody(t *testing.T) {
	in := `{"body":"Example log","traceid":"01020304000000000000000000000000","spanid":"0506070800000000","severity":"error","attributes":{"attr1":"1","attr2":"2"},"resources":{"host.name":"something"}}`

	out, err := encodeJSON(exampleLog())
	assert.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestConvertWithMapBody(t *testing.T) {
	in := `{"body":{"key1":"value","key2":"value"},"traceid":"01020304000000000000000000000000","spanid":"0506070800000000","severity":"error","attributes":{"attr1":"1","attr2":"2"},"resources":{"host.name":"something"}}`

	log, resource := exampleLog()
	mapVal := pdata.NewAttributeValueMap()
	mapVal.MapVal().Insert("key1", pdata.NewAttributeValueString("value"))
	mapVal.MapVal().Insert("key2", pdata.NewAttributeValueString("value"))
	mapVal.CopyTo(log.Body())

	out, err := encodeJSON(log, resource)
	assert.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestSerializeBody(t *testing.T) {

	arrayval := pdata.NewAttributeValueArray()
	arrayval.SliceVal().AppendEmpty().SetStringVal("a")
	arrayval.SliceVal().AppendEmpty().SetStringVal("b")

	simplemap := pdata.NewAttributeValueMap()
	simplemap.MapVal().InsertString("key", "val")

	complexmap := pdata.NewAttributeValueMap()
	complexmap.MapVal().InsertString("keystr", "val")
	complexmap.MapVal().InsertInt("keyint", 1)
	complexmap.MapVal().InsertDouble("keyint", 1)
	complexmap.MapVal().InsertBool("keybool", true)
	complexmap.MapVal().InsertNull("keynull")
	complexmap.MapVal().Insert("keyarr", arrayval)
	complexmap.MapVal().Insert("keymap", simplemap)
	complexmap.MapVal().Insert("keyempty", pdata.NewAttributeValueEmpty())

	testcases := []struct {
		input    pdata.AttributeValue
		expected []byte
	}{
		{
			pdata.NewAttributeValueEmpty(),
			nil,
		},
		{
			pdata.NewAttributeValueString("a"),
			[]byte(`"a"`),
		},
		{
			pdata.NewAttributeValueInt(1),
			[]byte(`1`),
		},
		{
			pdata.NewAttributeValueDouble(1.1),
			[]byte(`1.1`),
		},
		{
			pdata.NewAttributeValueBool(true),
			[]byte(`true`),
		},
		{
			simplemap,
			[]byte(`{"key":"val"}`),
		},
		{
			complexmap,
			[]byte(`{"keyarr":["a","b"],"keybool":true,"keyempty":null,"keyint":1,"keymap":{"key":"val"},"keynull":null,"keystr":"val"}`),
		},
		{
			arrayval,
			[]byte(`["a","b"]`),
		},
		{
			pdata.NewAttributeValueBytes([]byte(`abc`)),
			[]byte(`"YWJj"`),
		},
	}

	for _, test := range testcases {
		out, err := serializeBody(test.input)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, out)
	}
}
