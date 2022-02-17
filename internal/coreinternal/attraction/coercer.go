package attraction

import (
	"regexp"
	"strconv"

	"go.opentelemetry.io/collector/model/pdata"
)

var num = regexp.MustCompile(`^(\d+)(?:\.\d+)?$`)
var dub = regexp.MustCompile(`^(\d+(?:\.\d+)?)$`)

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
			n := num.FindStringSubmatch(s)
			if n != nil {
				intVal, _ := strconv.Atoi(n[1])
				v.SetIntVal(int64(intVal))
			} else {
				v.SetIntVal(int64(0))
			}
		default:
			v.SetIntVal(int64(0))
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
			n := dub.FindStringSubmatch(s)
			if n != nil {
				dubVal, _ := strconv.ParseFloat(n[1], 64)
				v.SetDoubleVal(dubVal)
			} else {
				v.SetDoubleVal(0)
			}
		default:
			v.SetDoubleVal(0)
		}
	default: // No-op
	}
}
