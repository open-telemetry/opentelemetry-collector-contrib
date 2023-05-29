// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"testing"

	"github.com/go-logfmt/logfmt"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func exampleLog() (plog.LogRecord, pcommon.Resource, pcommon.InstrumentationScope) {

	buffer := plog.NewLogRecord()
	buffer.Body().SetStr("Example log")
	buffer.SetSeverityText("error")
	buffer.Attributes().PutStr("attr1", "1")
	buffer.Attributes().PutStr("attr2", "2")
	buffer.SetTraceID([16]byte{1, 2, 3, 4})
	buffer.SetSpanID([8]byte{5, 6, 7, 8})

	resource := pcommon.NewResource()
	resource.Attributes().PutStr("host.name", "something")

	scope := pcommon.NewInstrumentationScope()
	scope.SetName("example-logger-name")
	scope.SetVersion("v1")

	return buffer, resource, scope
}

func TestEncodeJsonWithStringBody(t *testing.T) {
	in := `{"body":"Example log","traceid":"01020304000000000000000000000000","spanid":"0506070800000000","severity":"error","attributes":{"attr1":"1","attr2":"2"},"resources":{"host.name":"something"},"instrumentation_scope":{"name":"example-logger-name","version":"v1"}}`
	log, resource, scope := exampleLog()

	out, err := Encode(log, resource, scope)
	assert.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestEncodeJsonWithMapBody(t *testing.T) {
	in := `{"body":{"key1":"value","key2":"value"},"traceid":"01020304000000000000000000000000","spanid":"0506070800000000","severity":"error","attributes":{"attr1":"1","attr2":"2"},"resources":{"host.name":"something"},"instrumentation_scope":{"name":"example-logger-name","version":"v1"}}`

	log, resource, scope := exampleLog()
	mapVal := pcommon.NewValueMap()
	mapVal.Map().PutStr("key1", "value")
	mapVal.Map().PutStr("key2", "value")
	mapVal.CopyTo(log.Body())

	out, err := Encode(log, resource, scope)
	assert.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestSerializeComplexBody(t *testing.T) {

	arrayval := pcommon.NewValueSlice()
	arrayval.Slice().AppendEmpty().SetStr("a")
	arrayval.Slice().AppendEmpty().SetStr("b")

	simplemap := pcommon.NewValueMap()
	simplemap.Map().PutStr("key", "val")

	complexmap := pcommon.NewValueMap()
	complexmap.Map().PutStr("keystr", "val")
	complexmap.Map().PutInt("keyint", 1)
	complexmap.Map().PutDouble("keyint", 1)
	complexmap.Map().PutBool("keybool", true)
	complexmap.Map().PutEmpty("keynull")
	arrayval.CopyTo(complexmap.Map().PutEmpty("keyarr"))
	simplemap.CopyTo(complexmap.Map().PutEmpty("keymap"))
	complexmap.Map().PutEmpty("keyempty")

	bytes := pcommon.NewValueBytes()
	bytes.Bytes().FromRaw([]byte(`abc`))

	testcases := []struct {
		input          pcommon.Value
		expectedJSON   []byte
		expectedLogfmt []byte
	}{
		{
			pcommon.NewValueEmpty(),
			nil,
			nil,
		},
		{
			pcommon.NewValueStr("a"),
			[]byte(`"a"`),
			[]byte(`a=`),
		},
		{
			pcommon.NewValueInt(1),
			[]byte(`1`),
			[]byte(`msg=1`),
		},
		{
			pcommon.NewValueDouble(1.1),
			[]byte(`1.1`),
			[]byte(`msg=1.1`),
		},
		{
			pcommon.NewValueBool(true),
			[]byte(`true`),
			[]byte(`msg=true`),
		},
		{
			simplemap,
			[]byte(`{"key":"val"}`),
			[]byte(`key=val`),
		},
		{
			complexmap,
			[]byte(`{"keyarr":["a","b"],"keybool":true,"keyempty":null,"keyint":1,"keymap":{"key":"val"},"keynull":null,"keystr":"val"}`),
			[]byte(`keystr=val keyint=1 keybool=true keyarr_0=a keyarr_1=b keymap_key=val`),
		},
		{
			arrayval,
			[]byte(`["a","b"]`),
			[]byte(`body_0=a body_1=b`),
		},
		{
			bytes,
			[]byte(`"YWJj"`),
			[]byte(`msg=abc`),
		},
	}

	for _, test := range testcases {
		out, err := serializeBodyJSON(test.input)
		assert.NoError(t, err)
		assert.Equal(t, test.expectedJSON, out)

		keyvals := bodyToKeyvals(test.input)
		out, err = logfmt.MarshalKeyvals(keyvals...)
		assert.NoError(t, err)
		assert.Equal(t, string(test.expectedLogfmt), string(out))
	}
}

func TestEncodeLogfmtWithStringBody(t *testing.T) {
	in := `msg="hello world" traceID=01020304000000000000000000000000 spanID=0506070800000000 severity=error attribute_attr1=1 attribute_attr2=2 resource_host.name=something instrumentation_scope_name=example-logger-name instrumentation_scope_version=v1`
	log, resource, scope := exampleLog()
	log.Body().SetStr("msg=\"hello world\"")
	out, err := EncodeLogfmt(log, resource, scope)
	assert.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestEncodeLogfmtWithMapBody(t *testing.T) {
	in := `key1=value key2=value traceID=01020304000000000000000000000000 spanID=0506070800000000 severity=error attribute_attr1=1 attribute_attr2=2 resource_host.name=something instrumentation_scope_name=example-logger-name instrumentation_scope_version=v1`
	log, resource, scope := exampleLog()
	mapVal := pcommon.NewValueMap()
	mapVal.Map().PutStr("key1", "value")
	mapVal.Map().PutStr("key2", "value")
	mapVal.CopyTo(log.Body())
	out, err := EncodeLogfmt(log, resource, scope)
	assert.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestEncodeLogfmtWithSliceBody(t *testing.T) {
	in := `body_0=value body_1=true body_2=123 traceID=01020304000000000000000000000000 spanID=0506070800000000 severity=error attribute_attr1=1 attribute_attr2=2 resource_host.name=something instrumentation_scope_name=example-logger-name instrumentation_scope_version=v1`
	log, resource, scope := exampleLog()
	sliceVal := pcommon.NewValueSlice()
	sliceVal.Slice().AppendEmpty().SetStr("value")
	sliceVal.Slice().AppendEmpty().SetBool(true)
	sliceVal.Slice().AppendEmpty().SetInt(123)
	sliceVal.CopyTo(log.Body())
	out, err := EncodeLogfmt(log, resource, scope)
	assert.NoError(t, err)
	assert.Equal(t, in, out)
}

func TestEncodeLogfmtWithComplexAttributes(t *testing.T) {
	in := `Example= log= traceID=01020304000000000000000000000000 spanID=0506070800000000 severity=error attribute_attr1=1 attribute_attr2=2 attribute_aslice_0=fooo attribute_aslice_1_slice_0=true attribute_aslice_1_foo=bar attribute_aslice_2_nested="deeply nested" attribute_aslice_2_uint=123 resource_host.name=something resource_bslice_0=fooo resource_bslice_1_slice_0=true resource_bslice_1_foo=bar resource_bslice_2_nested="deeply nested" resource_bslice_2_uint=123 instrumentation_scope_name=example-logger-name instrumentation_scope_version=v1`
	log, resource, scope := exampleLog()
	sliceVal := pcommon.NewValueSlice()
	sliceVal.Slice().AppendEmpty().SetStr("fooo")
	map1 := pcommon.NewValueMap()
	map1.Map().PutEmptySlice("slice").AppendEmpty().SetBool(true)
	map1.Map().PutStr("foo", "bar")
	map1.CopyTo(sliceVal.Slice().AppendEmpty())
	anotherMap := pcommon.NewValueMap()
	anotherMap.Map().PutStr("nested", "deeply nested")
	anotherMap.Map().PutInt("uint", 123)
	anotherMap.CopyTo(sliceVal.Slice().AppendEmpty())
	sliceVal.CopyTo(log.Attributes().PutEmpty("aslice"))
	sliceVal.CopyTo(resource.Attributes().PutEmpty("bslice"))

	out, err := EncodeLogfmt(log, resource, scope)
	assert.NoError(t, err)
	assert.Equal(t, in, out)
}
