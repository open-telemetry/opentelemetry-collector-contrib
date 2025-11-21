package protocol

import (
	"fmt"
	"math"
	"strconv"
)

// Code taken from path_parser_helper.go
func parseCarbonTimestamp(tsStr string) (unixTime int64, unixTimeNs int64, err error) {
	unixTime, err = strconv.ParseInt(tsStr, 10, 64)
	if err == nil {
		return unixTime, 0, nil
	}

	var dblVal float64
	dblVal, err = strconv.ParseFloat(tsStr, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid carbon metric time: %w", err)
	}

	sec, frac := math.Modf(dblVal)
	unixTime = int64(sec)
	unixTimeNs = int64(frac * 1e9)
	return unixTime, unixTimeNs, nil
}

// Code taken from path_parser_helper.go
func parseCarbonValue(valueStr string) (intVal int64, dblVal float64, isFloat bool, err error) {
	intVal, err = strconv.ParseInt(valueStr, 10, 64)
	if err == nil {
		// Successfully parsed as int
		isFloat = false
		return intVal, 0, isFloat, nil
	}

	dblVal, err = strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid carbon metric value: %w", err)
	}

	isFloat = true
	return 0, dblVal, isFloat, nil
}
