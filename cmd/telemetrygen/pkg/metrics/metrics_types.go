// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"errors"
)

type metricType string

const (
	metricTypeGauge     = "Gauge"
	metricTypeSum       = "Sum"
	metricTypeHistogram = "Histogram"
)

// String is used both by fmt.Print and by Cobra in help text
func (e *metricType) String() string {
	return string(*e)
}

// Set must have pointer receiver so it doesn't change the value of a copy
func (e *metricType) Set(v string) error {
	switch v {
	case "Gauge", "Sum", "Histogram":
		*e = metricType(v)
		return nil
	default:
		return errors.New(`must be one of "Gauge", "Sum", "Histogram"`)
	}
}

// Type is only used in help text
func (e *metricType) Type() string {
	return "metricType"
}
