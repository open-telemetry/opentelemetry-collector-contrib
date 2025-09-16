// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package util // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/util"

import "strconv"

// Str2float64 converts a string to float64, handling dash values
func Str2float64(str string) float64 {
	if str == "-" {
		return 0 // Return 0 for dash values instead of -1
	}
	value, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0 // Return 0 for invalid values to match cisco_exporter behavior
	}
	return value
}

// Str2int64 converts a string to int64, handling dash values
func Str2int64(str string) int64 {
	if str == "-" {
		return 0 // Return 0 for dash values instead of -1
	}
	value, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return -1
	}
	return value
}
