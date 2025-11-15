package hydrolixexporter

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// convertAttributes converts OTLP attributes to array of maps
func convertAttributes(attrs pcommon.Map) []map[string]interface{} {
	var tags []map[string]interface{}

	attrs.Range(func(k string, v pcommon.Value) bool {
		tag := map[string]interface{}{
			k: attributeValueToInterface(v),
		}
		tags = append(tags, tag)
		return true
	})

	return tags
}

// attributeValueToInterface converts OTLP attribute value to interface{}
func attributeValueToInterface(v pcommon.Value) interface{} {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeInt:
		return fmt.Sprintf("%d", v.Int())
	case pcommon.ValueTypeDouble:
		return fmt.Sprintf("%f", v.Double())
	case pcommon.ValueTypeBool:
		return fmt.Sprintf("%t", v.Bool())
	case pcommon.ValueTypeBytes:
		return v.Bytes().AsRaw()
	default:
		return v.AsString()
	}
}

// extractStringAttr extracts a string attribute value by key
func extractStringAttr(attrs pcommon.Map, key string) string {
	if val, ok := attrs.Get(key); ok {
		return val.AsString()
	}
	return ""
}
