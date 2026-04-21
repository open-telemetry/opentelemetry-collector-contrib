// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dpfilters // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"

// PropertyFilter is a collection of *StringFilter items used in determining if a given property (name and value)
// should be included with a dimension update request. The default values for all fields is equivalent to the regex
// StringFilter `/^.*$/` to match with any potential value.
//
// Examples:
// Don't send any dimension updates for `k8s.pod.uid` dimension:
// - dimension_name: "k8s.pod.uid"
// Don't send dimension updates for any dimension with a value of `some.value`:
// - dimension_value: "some.value"
// Don't send dimension updates including a `some.property` property for any dimension:
// - property_name: "some.property"
// Don't send dimension updates including a `some.property` property with a "some.value" value for any dimension
//   - property_name: "some.property"
//     property_value: "some.value"
type PropertyFilter struct {
	// PropertyName is the (inverted) literal, regex, or globbed property name/key to not include in dimension updates
	PropertyName *StringFilter `mapstructure:"property_name"`
	// PropertyValue is the (inverted) literal or globbed property value to not include in dimension updates
	PropertyValue *StringFilter `mapstructure:"property_value"`
	// DimensionName is the (inverted) literal, regex, or globbed dimension name/key to not target for dimension updates.
	// If there are no sub-property filters for its enclosing entry, it will disable dimension updates
	// for this dimension name in total.
	DimensionName *StringFilter `mapstructure:"dimension_name"`
	// PropertyValue is the (inverted) literal, regex, or globbed dimension value to not target with a dimension update
	// If there are no sub-property filters for its enclosing entry, it will disable dimension updates
	// for this dimension value in total.
	DimensionValue *StringFilter `mapstructure:"dimension_value"`
}
