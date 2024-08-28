package pdatautil

import "go.opentelemetry.io/collector/pdata/pcommon"

type Dimension struct {
	Name  string
	Value *pcommon.Value
}

// GetDimensionValue gets the Dimension Value for the given configured Dimension.
// It iterates over multiple attributes until a Value is found.
// The order comes first, the higher the priority.
// Finally, falls back to the configured default Value if provided.
//
// The ok flag indicates if a Dimension Value was fetched in order to differentiate
// an empty string Value from a state where no Value was found.
func GetDimensionValue(d Dimension, attributes ...pcommon.Map) (v pcommon.Value, ok bool) {
	// The more specific span attribute should take precedence.
	for _, attrs := range attributes {
		if attr, exists := attrs.Get(d.Name); exists {
			return attr, true
		}
	}
	// Set the default if configured, otherwise this metric will have no Value set for the Dimension.
	if d.Value != nil {
		return *d.Value, true
	}
	return v, ok
}

func GetAttributeValue(key string, attributes ...pcommon.Map) (string, bool) {
	for _, attr := range attributes {
		if v, ok := attr.Get(key); ok {
			return v.AsString(), true
		}
	}
	return "", false
}
