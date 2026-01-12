package ottlruntime

// NoneValue represents a value type that means nothing(void). Used for Expr that do not return any value.
type NoneValue interface {
	Value
}

type noneValue struct{}

func (n noneValue) IsNil() bool {
	return false
}

func (n noneValue) unexportedFunc() {}

// NewNoneValue returns a None Value. Used for Expr that do not return any value.
func NewNoneValue() NoneValue {
	return noneValue{}
}
