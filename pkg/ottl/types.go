package ottl

// ValueType represents the compile-time type of an OTTL expression.
type ValueType int

const (
	TypeUnknown ValueType = iota
	TypeString
	TypeInt
	TypeFloat
	TypeBool
	TypeMap
	TypeSlice
)
