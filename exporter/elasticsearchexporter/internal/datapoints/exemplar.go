// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"errors"
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func ExemplarValue(e pmetric.Exemplar) (pcommon.Value, error) {
	switch e.ValueType() {
	case pmetric.ExemplarValueTypeDouble:
		value := e.DoubleValue()
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return pcommon.Value{}, errors.New("invalid double value in Exemplar, either NaN or Inf")
		}
		return pcommon.NewValueDouble(value), nil
	case pmetric.ExemplarValueTypeInt:
		return pcommon.NewValueInt(e.IntValue()), nil
	default:
		return pcommon.Value{}, errors.New("exemplar has no value")
	}
}
