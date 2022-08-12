package tql

import (
	"bytes"
)

// The functions in this file implement a general-purpose comparison of two
// values of type any, which for the purposes of TQL mean values that are one of
// int, float, string, bool, or pointers to those, or []byte, or nil.

// invalidComparison returns false for everything except NE (where it returns true to indicate that the
// objects were definitely not equivalent).
// It also gives us an opportunity to log something first if we want to do that.
func invalidComparison(s string, op compareOp) bool {
	return op == NE
}

type compareOp int

const (
	EQ compareOp = iota
	NE
	LT
	LTE
	GTE
	GT
)

// String for compareOp gives us more legible test results and error messages.
func (op compareOp) String() string {
	switch op {
	case EQ:
		return "EQ"
	case NE:
		return "NE"
	case LT:
		return "LT"
	case LTE:
		return "LTE"
	case GTE:
		return "GTE"
	case GT:
		return "GT"
	default:
		return "UNKNOWN OP!"
	}
}

// TODO: Many of these compare* functions for base types could be generic,
// but we can't do that until we've committed to go 1.18.

func compareBools(a bool, b bool, op compareOp) bool {
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

func compareInt64s(a int64, b int64, op compareOp) bool {
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

func compareFloat64s(a float64, b float64, op compareOp) bool {
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

func compareStrings(a string, b string, op compareOp) bool {
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

func compareBytes(a []byte, b []byte, op compareOp) bool {
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

func compareBool(a bool, b any, op compareOp) bool {
	switch v := b.(type) {
	case bool:
		return compareBools(a, v, op)
	case *bool:
		if v == nil {
			return op == NE
		}
		return compareBools(a, *v, op)
	default:
		return invalidComparison("bool to non-bool", op)
	}
}

func compareString(a string, b any, op compareOp) bool {
	switch v := b.(type) {
	case string:
		return compareStrings(a, v, op)
	case *string:
		if v == nil {
			return op == NE
		}
		return compareStrings(a, *v, op)
	default:
		return invalidComparison("string to non-string", op)
	}
}

func compareByte(a []byte, b any, op compareOp) bool {
	switch v := b.(type) {
	case []byte:
		return compareBytes(a, v, op)
	default:
		return invalidComparison("Bytes to non-Bytes", op)
	}
}

func compareInt64(a int64, b any, op compareOp) bool {
	switch v := b.(type) {
	case int:
		return compareInt64s(a, int64(v), op)
	case int32:
		return compareInt64s(a, int64(v), op)
	case int64:
		return compareInt64s(a, int64(v), op)
	case float32:
		return compareFloat64s(float64(a), float64(v), op)
	case float64:
		return compareFloat64s(float64(a), v, op)
	case *int:
		if v == nil {
			return op == NE
		}
		return compareInt64s(a, int64(*v), op)
	case *int32:
		if v == nil {
			return op == NE
		}
		return compareInt64s(a, int64(*v), op)
	case *int64:
		if v == nil {
			return op == NE
		}
		return compareInt64s(a, int64(*v), op)
	case *float32:
		if v == nil {
			return op == NE
		}
		return compareFloat64s(float64(a), float64(*v), op)
	case *float64:
		if v == nil {
			return op == NE
		}
		return compareFloat64s(float64(a), *v, op)
	default:
		return invalidComparison("int to non-numeric value", op)
	}
}

func compareFloat64(a float64, b any, op compareOp) bool {
	switch v := b.(type) {
	case int:
		return compareFloat64s(a, float64(v), op)
	case int32:
		return compareFloat64s(a, float64(v), op)
	case int64:
		return compareFloat64s(a, float64(v), op)
	case float32:
		return compareFloat64s(a, float64(v), op)
	case float64:
		return compareFloat64s(a, v, op)
	case *int:
		if v == nil {
			return op == NE
		}
		return compareFloat64s(a, float64(*v), op)

	case *int32:
		if v == nil {
			return op == NE
		}
		return compareFloat64s(a, float64(*v), op)

	case *int64:
		if v == nil {
			return op == NE
		}
		return compareFloat64s(a, float64(*v), op)

	case *float32:
		if v == nil {
			return op == NE
		}
		return compareFloat64s(a, float64(*v), op)

	case *float64:
		if v == nil {
			return op == NE
		}
		return compareFloat64s(a, *v, op)
	default:
		return invalidComparison("float to non-numeric value", op)
	}
}

// a and b are the return values from a Getter; we try to compare them
// according to the given operator.
func compare(a any, b any, op compareOp) bool {
	// nils are equal to each other and never equal to anything else,
	// so if they're both nil, report equality.
	if a == nil && b == nil {
		return (op == EQ || op == LTE || op == GTE)
	}
	// otherwise if either is nil, they're definitely not equal.
	if a == nil || b == nil {
		return op == NE
	}
	// Anything else, we switch on the left side first.
	switch v := a.(type) {
	case bool:
		return compareBool(v, b, op)
	case int:
		return compareInt64(int64(v), b, op)
	case int32:
		return compareInt64(int64(v), b, op)
	case int64:
		return compareInt64(v, b, op)
	case float32:
		return compareFloat64(float64(v), b, op)
	case float64:
		return compareFloat64(v, b, op)
	case string:
		return compareString(v, b, op)
	case []byte:
		return compareByte(v, b, op)
	case *bool:
		if v == nil {
			return compare(nil, b, op)
		}
		return compareBool(*v, b, op)
	case *int:
		if v == nil {
			return compare(nil, b, op)
		}
		return compareInt64(int64(*v), b, op)
	case *int32:
		if v == nil {
			return compare(nil, b, op)
		}
		return compareInt64(int64(*v), b, op)
	case *int64:
		if v == nil {
			return compare(nil, b, op)
		}
		return compareInt64(*v, b, op)
	case *float32:
		if v == nil {
			return compare(nil, b, op)
		}
		return compareFloat64(float64(*v), b, op)
	case *float64:
		if v == nil {
			return compare(nil, b, op)
		}
		return compareFloat64(*v, b, op)
	case *string:
		if v == nil {
			return compare(nil, b, op)
		}
		return compareString(*v, b, op)
	default:
		return invalidComparison("unsupported type on left", op)
	}
}
