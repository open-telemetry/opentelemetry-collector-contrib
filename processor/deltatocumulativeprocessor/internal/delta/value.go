package delta

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type ValueType = pmetric.NumberDataPointValueType

const (
	TypeInt   = pmetric.NumberDataPointValueTypeInt
	TypeFloat = pmetric.NumberDataPointValueTypeDouble
)

type Value struct {
	Type  ValueType
	Float float64
	Int   int64

	Start pcommon.Timestamp
	Last  pcommon.Timestamp
}
