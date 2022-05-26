package servicegraphprocessor

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.9.0"
)

func findAttributeValue(key string, attributes ...pcommon.Map) (string, bool) {
	for _, attr := range attributes {
		if v, ok := attr.Get(key); ok {
			return v.AsString(), true
		}
	}
	return "", false
}

func findServiceName(attributes pcommon.Map) (string, bool) {
	return findAttributeValue(semconv.AttributeServiceName, attributes)
}
