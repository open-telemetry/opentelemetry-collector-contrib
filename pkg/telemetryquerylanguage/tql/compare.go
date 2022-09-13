// Copyright  The OpenTelemetry Authors
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

package tql // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"

import (
	"bytes"
	"fmt"

	"golang.org/x/exp/constraints"
)

// The functions in this file implement a general-purpose comparison of two
// values of type any, which for the purposes of TQL mean values that are one of
// int, float, string, bool, or pointers to those, or []byte, or nil.

// invalidComparison returns false for everything except NE (where it returns true to indicate that the
// objects were definitely not equivalent).
// It also gives us an opportunity to log something.
func invalidComparison(msg string, op CompareOp) bool {
	fmt.Printf("%s with op %v\n", msg, op)
	return op == NE
}

// comparePrimitives implements a generic comparison helper for all Ordered types (derived from Float, Int, or string).
// According to benchmarks, it's faster than explicit comparison functions for these types.
func comparePrimitives[T constraints.Ordered](a T, b T, op CompareOp) bool {
	switch op {
	case EQ:
		return a == b
	case NE:
		return a != b
	case LT:
		return a < b
	case LTE:
		return a <= b
	case GTE:
		return a >= b
	case GT:
		return a > b
	default:
		return false
	}
}

func compareBools(a bool, b bool, op CompareOp) bool {
	switch op {
	case EQ:
		return a == b
	case NE:
		return a != b
	case LT:
		return !a && b
	case LTE:
		return !a || b
	case GTE:
		return a || !b
	case GT:
		return a && !b
	default:
		return false
	}
}

func compareBytes(a []byte, b []byte, op CompareOp) bool {
	switch op {
	case EQ:
		return bytes.Equal(a, b)
	case NE:
		return !bytes.Equal(a, b)
	case LT:
		return bytes.Compare(a, b) < 0
	case LTE:
		return bytes.Compare(a, b) <= 0
	case GTE:
		return bytes.Compare(a, b) >= 0
	case GT:
		return bytes.Compare(a, b) > 0
	default:
		return false
	}
}

func compareBool(a bool, b any, op CompareOp) bool {
	switch v := b.(type) {
	case bool:
		return compareBools(a, v, op)
	default:
		return invalidComparison("bool to non-bool", op)
	}
}

func compareString(a string, b any, op CompareOp) bool {
	switch v := b.(type) {
	case string:
		return comparePrimitives(a, v, op)
	default:
		return invalidComparison("string to non-string", op)
	}
}

func compareByte(a []byte, b any, op CompareOp) bool {
	switch v := b.(type) {
	case nil:
		return op == NE
	case []byte:
		if v == nil {
			return op == NE
		}
		return compareBytes(a, v, op)
	default:
		return invalidComparison("Bytes to non-Bytes", op)
	}
}

func compareInt64(a int64, b any, op CompareOp) bool {
	switch v := b.(type) {
	case int64:
		return comparePrimitives(a, v, op)
	case float64:
		return comparePrimitives(float64(a), v, op)
	default:
		return invalidComparison("int to non-numeric value", op)
	}
}

func compareFloat64(a float64, b any, op CompareOp) bool {
	switch v := b.(type) {
	case int64:
		return comparePrimitives(a, float64(v), op)
	case float64:
		return comparePrimitives(a, v, op)
	default:
		return invalidComparison("float to non-numeric value", op)
	}
}

// a and b are the return values from a Getter; we try to compare them
// according to the given operator.
func compare(a any, b any, op CompareOp) bool {
	// nils are equal to each other and never equal to anything else,
	// so if they're both nil, report equality.
	if a == nil && b == nil {
		return op == EQ || op == LTE || op == GTE
	}
	// Anything else, we switch on the left side first.
	switch v := a.(type) {
	case nil:
		// If a was nil, it means b wasn't and inequalities don't apply,
		// so let's swap and give it the chance to get evaluated.
		return compare(b, nil, op)
	case bool:
		return compareBool(v, b, op)
	case int64:
		return compareInt64(v, b, op)
	case float64:
		return compareFloat64(v, b, op)
	case string:
		return compareString(v, b, op)
	case []byte:
		if v == nil {
			return compare(b, nil, op)
		}
		return compareByte(v, b, op)
	default:
		// If we don't know what type it is, we can't do inequalities yet. So we can fall back to the old behavior where we just
		// use Go's standard equality.
		switch op {
		case EQ:
			return a == b
		case NE:
			return a != b
		default:
			return invalidComparison("unsupported type for inequality on left", op)
		}
	}
}
