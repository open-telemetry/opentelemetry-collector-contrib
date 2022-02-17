package attraction

import (
	"strconv"

	"go.opentelemetry.io/collector/model/pdata"
)

func convertValue(to string, v pdata.AttributeValue) {
	switch to {
	case "string":
		switch v.Type().String() {
		case "STRING":
		default:
			v.SetStringVal(v.AsString())
		}
	case "int":
		switch v.Type().String() {
		case "INT":
		case "DOUBLE":
			v.SetIntVal(int64(v.DoubleVal()))
		case "BOOL":
			if v.BoolVal() {
				v.SetIntVal(1)
			} else {
				v.SetIntVal(0)
			}
		case "STRING":
			s := v.StringVal()
			n, err := strconv.ParseInt(s, 10, 64)
			if err == nil {
				v.SetIntVal(n)
			} // else leave original value
		default: // leave original value
		}
	case "double":
		switch v.Type().String() {
		case "INT":
			v.SetDoubleVal(float64(v.IntVal()))
		case "DOUBLE":
		case "BOOL":
			if v.BoolVal() {
				v.SetDoubleVal(1)
			} else {
				v.SetDoubleVal(0)
			}
		case "STRING":
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
