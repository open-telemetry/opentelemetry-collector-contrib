package delta

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/identity"
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

	Start time.Time
	Last  time.Time
}

type Metric struct {
	series map[identity.Series]*Value
}
