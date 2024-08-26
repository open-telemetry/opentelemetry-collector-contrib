// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregateutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

// AggregationType is the enum to capture the three types of aggregation for the aggregation operation.
type AggregationType string

const (
	// Sum indicates taking the sum of the aggregated data.
	Sum AggregationType = "sum"

	// Mean indicates taking the mean of the aggregated data.
	Mean AggregationType = "mean"

	// Min indicates taking the minimum of the aggregated data.
	Min AggregationType = "min"

	// Max indicates taking the max of the aggregated data.
	Max AggregationType = "max"

	// Median indicates taking the median of the aggregated data.
	Median AggregationType = "median"

	// Count indicates taking the count of the aggregated data.
	Count AggregationType = "count"
)

var AggregationTypes = []AggregationType{Sum, Mean, Min, Max, Median, Count}

func (at AggregationType) IsValid() bool {
	for _, aggregationType := range AggregationTypes {
		if at == aggregationType {
			return true
		}
	}

	return false
}

func GetSupportedAggregationFunctionsList() string {
	slice := make([]string, 0, len(AggregationTypes))
	for _, a := range AggregationTypes {
		slice = append(slice, string(a))
	}
	return strings.Join(slice, ", ")
}

type AggGroups struct {
	gauge        map[string]pmetric.NumberDataPointSlice
	sum          map[string]pmetric.NumberDataPointSlice
	histogram    map[string]pmetric.HistogramDataPointSlice
	expHistogram map[string]pmetric.ExponentialHistogramDataPointSlice
}

func ConvertToAggregationFunction(str string) (AggregationType, error) {
	a := AggregationType(str)
	if a.IsValid() {
		return a, nil
	}
	return a, fmt.Errorf("unsupported function: '%s'", str)
}
