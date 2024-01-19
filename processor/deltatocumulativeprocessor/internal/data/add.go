package data

import "go.opentelemetry.io/collector/pdata/pmetric"

func (dp Number) Add(in Number) Number {
	switch in.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		v := dp.DoubleValue() + in.DoubleValue()
		dp.SetDoubleValue(v)
	case pmetric.NumberDataPointValueTypeInt:
		v := dp.IntValue() + in.IntValue()
		dp.SetIntValue(v)
	}
	dp.SetTimestamp(in.Timestamp())
	return dp
}

func (dp Histogram) Add(in Histogram) Histogram {
	panic("todo")
}

func (dp ExpHistogram) Add(in ExpHistogram) ExpHistogram {
	panic("todo")
}
