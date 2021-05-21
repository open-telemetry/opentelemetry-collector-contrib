package lokiexporter

import (
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/collector/consumer/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
)

type jsonRecord struct {
	Name       string                 `json:"name,omitempty"`
	Body       string                 `json:"body,omitempty"`
	TraceID    string                 `json:"traceid,omitempty"`
	SpanID     string                 `json:"spanid,omitempty"`
	Severity   string                 `json:"severity,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

func encodeJSON(lr pdata.LogRecord) string {
	// json1 := jsonRecord{lr.Name(), lr.Body().StringVal(), lr.TraceID(), lr.SpanID(), lr.SeverityText()}
	var json1 jsonRecord
	var json2 []byte

	json1 = jsonRecord{Name: lr.Name(), Body: lr.Body().StringVal(), TraceID: lr.TraceID().HexString(), SpanID: lr.SpanID().HexString(), Severity: lr.SeverityText(), Attributes: tracetranslator.AttributeMapToMap(lr.Attributes())}
	// json = {name: lr.Name(), body: lr.Body().StringVal(), traceid: lr.TraceID().StringVal(), spanid: lr.SpanID().StringVal(), severity: lr.Severity().StringVal(), attributes: {lr.AttributeMap().Get()}}
	fmt.Printf("JSON 1: %v", json1)

	json2, err := json.Marshal(json1)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Println("JSON 2" + string(json2))
	return string(json2)
}
