package lokiexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func exampleLog() pdata.LogRecord {

	buffer := pdata.NewLogRecord()
	buffer.Body().SetStringVal("Example log")
	buffer.SetName("name")
	buffer.SetSeverityText("error")
	buffer.Attributes().Insert("attr1", pdata.NewAttributeValueString("1"))
	buffer.Attributes().Insert("attr2", pdata.NewAttributeValueString("2"))
	buffer.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
	buffer.SetSpanID(pdata.NewSpanID([8]byte{5, 6, 7, 8}))

	return buffer
}

func exampleJSON() string {
	jsonExample := `{"name":"name","body":"Example log","traceid":"01020304000000000000000000000000","spanid":"0506070800000000","severity":"error","attributes":{"attr1":"1","attr2":"2"}}`
	return jsonExample
}

func TestConvert(t *testing.T) {
	in := exampleJSON()
	out, err := encodeJSON(exampleLog())
	t.Log(in)
	t.Log(out, err)
	assert.Equal(t, in, out)
}
