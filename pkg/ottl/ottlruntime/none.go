// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

// NoneValue represents a value type that means nothing(void). Used for Expr that do not return any value.
type NoneValue struct {
	value[struct{}]
}

// NewNoneValue returns a None Value. Used for Expr that do not return any value.
func NewNoneValue() NoneValue {
	return NoneValue{value: newValue(struct{}{}, false)}
}
