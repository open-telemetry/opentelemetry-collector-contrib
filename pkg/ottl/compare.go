// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/runtime"
)

// ValueComparator defines methods for comparing values using the OTTL comparison rules
// (https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/LANGUAGE.md#comparison-rules)
type ValueComparator interface {
	// Equal compares two values for equality, returning true if they are equals
	// according to the OTTL comparison rules.
	Equal(a, b any) bool
	// NotEqual compares two values for equality, returning true if they are different
	// according to the OTTL comparison rules.
	NotEqual(a, b any) bool
	// Less compares two values, returning true if the first value is less than the second
	// value, using the OTTL comparison rules.
	Less(a, b any) bool
	// LessEqual compares two values, returning true if the first value is less or equal
	// to the second value, using the OTTL comparison rules.
	LessEqual(a, b any) bool
	// Greater compares two values, returning true if the first value is greater than the
	// second value, using the OTTL comparison rules.
	Greater(a, b any) bool
	// GreaterEqual compares two values, returning true if the first value is greater or
	// equal to the second value, using the OTTL comparison rules.
	GreaterEqual(a, b any) bool
}

// NewValueComparator creates a new ValueComparator instance that can be used to compare
// values using the OTTL comparison rules.
func NewValueComparator() ValueComparator {
	return runtime.NewValueComparator()
}
