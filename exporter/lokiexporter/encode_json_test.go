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
	buffer.SetName("name")
	buffer.SetSeverityText("error")
	buffer.Attributes().Insert("attr1", pdata.NewAttributeValueString("1"))
	buffer.Attributes().Insert("attr2", pdata.NewAttributeValueString("2"))
	buffer.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
	buffer.SetSpanID(pdata.NewSpanID([8]byte{5, 6, 7, 8}))

	resource := pdata.NewResource()
	resource.Attributes().Insert("host.name", pdata.NewAttributeValueString("something"))

	return buffer, resource
}

func exampleJSON() string {
	jsonExample := `{"name":"name","body":"Example log","traceid":"01020304000000000000000000000000","spanid":"0506070800000000","severity":"error","attributes":{"attr1":"1","attr2":"2"},"resources":{"host.name":"something"}}`
	return jsonExample
}

func TestConvertString(t *testing.T) {
	in := exampleJSON()
	out, err := encodeJSON(exampleLog())
	t.Log(in)
	t.Log(out, err)
	assert.Equal(t, in, out)
}

func TestConvertNonString(t *testing.T) {
	in := exampleJSON()
	log, resource := exampleLog()
	mapVal := pdata.NewAttributeValueMap()
	mapVal.MapVal().Insert("key1", pdata.NewAttributeValueString("value"))
	mapVal.MapVal().Insert("key2", pdata.NewAttributeValueString("value"))
	mapVal.CopyTo(log.Body())

	out, err := encodeJSON(log, resource)
	t.Log(in)
	t.Log(out, err)
	assert.EqualError(t, err, "unsuported body type to serialize")
}
