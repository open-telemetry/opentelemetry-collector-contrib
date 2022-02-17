package attraction

import (
	"strconv"

	"go.opentelemetry.io/collector/model/pdata"
)

const (
	stringConversionTarget = "string"
	intConversionTarget    = "int"
	doubleConversionTarget = "double"
)

func convertValue(to string, v pdata.AttributeValue) {
	switch to {
	case stringConversionTarget:
		switch v.Type() {
		case pdata.AttributeValueTypeString:
		default:
			v.SetStringVal(v.AsString())
		}
	case intConversionTarget:
		switch v.Type() {
		case pdata.AttributeValueTypeInt:
		case pdata.AttributeValueTypeDouble:
			v.SetIntVal(int64(v.DoubleVal()))
		case pdata.AttributeValueTypeBool:
			if v.BoolVal() {
				v.SetIntVal(1)
			} else {
				v.SetIntVal(0)
			}
		case pdata.AttributeValueTypeString:
			s := v.StringVal()
			n, err := strconv.ParseInt(s, 10, 64)
			if err == nil {
				v.SetIntVal(n)
			} // else leave original value
		default: // leave original value
		}
	case doubleConversionTarget:
		switch v.Type() {
		case pdata.AttributeValueTypeInt:
			v.SetDoubleVal(float64(v.IntVal()))
		case pdata.AttributeValueTypeDouble:
		case pdata.AttributeValueTypeBool:
			if v.BoolVal() {
				v.SetDoubleVal(1)
			} else {
				v.SetDoubleVal(0)
			}
		case pdata.AttributeValueTypeString:
			s := v.StringVal()
			n, err := strconv.ParseFloat(s, 64)
			if err == nil {
				v.SetDoubleVal(n)
			} // else leave original value
		default: // leave original value
		}
	default: // No-op
	}
}
