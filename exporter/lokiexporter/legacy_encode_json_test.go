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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func exampleLog() (plog.LogRecord, pcommon.Resource) {

	buffer := plog.NewLogRecord()
	buffer.Body().SetStringVal("Example log")
	buffer.SetSeverityText("error")
	buffer.Attributes().UpsertString("attr1", "1")
	buffer.Attributes().UpsertString("attr2", "2")
	buffer.SetTraceID([16]byte{1, 2, 3, 4})
	buffer.SetSpanID([8]byte{5, 6, 7, 8})

	resource := pcommon.NewResource()
	resource.Attributes().UpsertString("host.name", "something")

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
	mapVal := pcommon.NewValueMap()
	mapVal.MapVal().UpsertString("key1", "value")
	mapVal.MapVal().UpsertString("key2", "value")
	mapVal.CopyTo(log.Body())

	out, err := encodeJSON(log, resource)
	assert.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestSerializeBody(t *testing.T) {

	arrayval := pcommon.NewValueSlice()
	arrayval.SliceVal().AppendEmpty().SetStringVal("a")
	arrayval.SliceVal().AppendEmpty().SetStringVal("b")

	simplemap := pcommon.NewValueMap()
	simplemap.MapVal().UpsertString("key", "val")

	complexmap := pcommon.NewValueMap()
	complexmap.MapVal().UpsertString("keystr", "val")
	complexmap.MapVal().UpsertInt("keyint", 1)
	complexmap.MapVal().UpsertDouble("keyint", 1)
	complexmap.MapVal().UpsertBool("keybool", true)
	complexmap.MapVal().UpsertEmpty("keynull")
	arrayval.CopyTo(complexmap.MapVal().UpsertEmpty("keyarr"))
	simplemap.CopyTo(complexmap.MapVal().UpsertEmpty("keymap"))
	complexmap.MapVal().UpsertEmpty("keyempty")

	bytes := pcommon.NewValueBytesEmpty()
	bytes.BytesVal().FromRaw([]byte(`abc`))

	testcases := []struct {
		input    pcommon.Value
		expected []byte
	}{
		{
			pcommon.NewValueEmpty(),
			nil,
		},
		{
			pcommon.NewValueString("a"),
			[]byte(`"a"`),
		},
		{
			pcommon.NewValueInt(1),
			[]byte(`1`),
		},
		{
			pcommon.NewValueDouble(1.1),
			[]byte(`1.1`),
		},
		{
			pcommon.NewValueBool(true),
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
			bytes,
			[]byte(`"YWJj"`),
		},
	}

	for _, test := range testcases {
		out, err := serializeBody(test.input)
		assert.NoError(t, err)
		assert.Equal(t, test.expected, out)
	}
}
