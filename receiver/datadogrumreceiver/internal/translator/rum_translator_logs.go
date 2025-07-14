package translator

import (
	"net/http"

	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
)

func setLogAttributes(flatPayload map[string]any, logRecord plog.LogRecord) {
    for rumKey, meta := range attributeMetaMap {
        val, exists := flatPayload[rumKey]
        if !exists {
            continue
        }

        switch meta.Type {
        case StringAttribute:
            if s, ok := val.(string); ok {
                logRecord.Attributes().PutStr(meta.OTLPName, s)
            }
        case BoolAttribute:
            if b, ok := val.(bool); ok {
                logRecord.Attributes().PutBool(meta.OTLPName, b)
            }
        case NumberAttribute:
            if f, ok := val.(float64); ok {
                logRecord.Attributes().PutDouble(meta.OTLPName, f)
            }
		case IntegerAttribute:
			if i, ok := val.(int); ok {
				logRecord.Attributes().PutInt(meta.OTLPName, int64(i))
			}
        case ObjectAttribute:
            if o, ok := val.(map[string]any); ok {
				objVal := logRecord.Attributes().PutEmptyMap(meta.OTLPName)
                for k, v := range o {
                    objVal.PutStr(meta.OTLPName + k, v.(string))
                }
            }
        case ArrayAttribute:
            if a, ok := val.([]any); ok {
				arrVal := logRecord.Attributes().PutEmptySlice(meta.OTLPName)
                for _, v := range a {
					arrVal.AppendEmpty().SetStr(v.(string))
				}
            }
        case StringOrArrayAttribute:
            if s, ok := val.(string); ok {
                logRecord.Attributes().PutStr(meta.OTLPName, s)
            } else if a, ok := val.([]any); ok {
				arrVal := logRecord.Attributes().PutEmptySlice(meta.OTLPName)
                for _, v := range a {
					arrVal.AppendEmpty().SetStr(v.(string))
				}
            }
        }
    }
}

func ToLogs(payload map[string]any, req *http.Request, reqBytes []byte) plog.Logs {

	results := plog.NewLogs()
	rl := results.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl(semconv.SchemaURL)
	parseRUMRequestIntoResource(rl.Resource(), payload, req, reqBytes)

	in := rl.ScopeLogs().AppendEmpty()
	in.Scope().SetName(InstrumentationScopeName)

	newLogRecord := in.LogRecords().AppendEmpty()
	
	flatPayload := flattenJSON(payload)

    setLogAttributes(flatPayload, newLogRecord)

	return results
}
