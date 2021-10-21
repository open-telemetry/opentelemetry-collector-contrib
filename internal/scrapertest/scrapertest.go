// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scrapertest

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
)

// CompareMetrics compares each part of two given metric slices and returns
// an error if they don't match. The error describes what didn't match.
func CompareMetricSlices(expected, actual pdata.MetricSlice) error {
	if actual.Len() != expected.Len() {
		return fmt.Errorf("metric slices not of same length")
	}

	actualByName := make(map[string]pdata.Metric, actual.Len())
	for i := 0; i < actual.Len(); i++ {
		a := actual.At(i)
		actualByName[a.Name()] = a
	}

	expectedByName := make(map[string]pdata.Metric, expected.Len())
	for i := 0; i < expected.Len(); i++ {
		e := expected.At(i)
		expectedByName[e.Name()] = e
	}

	var errs error
	matchedMetrics := make(map[pdata.Metric]pdata.Metric, expected.Len())
	for name, am := range actualByName {
		em, ok := expectedByName[name]
		if !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected metric %s", name))
		} else {
			matchedMetrics[am] = em
		}
	}
	for name := range expectedByName {
		if _, ok := actualByName[name]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("missing expected metric %s", name))
		}
	}

	if errs != nil {
		return errs
	}

	for am, em := range matchedMetrics {
		if am.Name() != em.Name() {
			return fmt.Errorf("metric name does not match expected: %s, actual: %s", em.Name(), am.Name())
		}
		if am.DataType() != em.DataType() {
			return fmt.Errorf("metric datatype does not match expected: %s, actual: %s", em.DataType(), am.DataType())
		}
		if am.Description() != em.Description() {
			return fmt.Errorf("metric description does not match expected: %s, actual: %s", em.Description(), am.Description())
		}
		if am.Unit() != em.Unit() {
			return fmt.Errorf("metric Unit does not match expected: %s, actual: %s", em.Unit(), am.Unit())
		}

		var actualDataPoints pdata.NumberDataPointSlice
		var expectedDataPoints pdata.NumberDataPointSlice

		switch am.DataType() {
		case pdata.MetricDataTypeGauge:
			actualDataPoints = am.Gauge().DataPoints()
			expectedDataPoints = em.Gauge().DataPoints()
		case pdata.MetricDataTypeSum:
			if am.Sum().AggregationTemporality() != em.Sum().AggregationTemporality() {
				return fmt.Errorf("metric AggregationTemporality does not match expected: %s, actual: %s", em.Sum().AggregationTemporality(), am.Sum().AggregationTemporality())
			}
			if am.Sum().IsMonotonic() != em.Sum().IsMonotonic() {
				return fmt.Errorf("metric IsMonotonic does not match expected: %t, actual: %t", em.Sum().IsMonotonic(), am.Sum().IsMonotonic())
			}
			actualDataPoints = am.Sum().DataPoints()
			expectedDataPoints = em.Sum().DataPoints()
		}

		if actualDataPoints.Len() != expectedDataPoints.Len() {
			return fmt.Errorf("length of datapoints don't match, metric name: %s", am.Name())
		}

		var errs error
		for j := 0; j < expectedDataPoints.Len(); j++ {
			edp := expectedDataPoints.At(j)
			var foundMatch bool
			for k := 0; k < actualDataPoints.Len(); k++ {
				adp := actualDataPoints.At(k)
				if reflect.DeepEqual(adp.Attributes().Sort().AsRaw(), edp.Attributes().Sort().AsRaw()) {
					foundMatch = true
					break
				}
			}

			if !foundMatch {
				errs = multierr.Append(errs, fmt.Errorf("metric %s missing expected data point: Labels: %v", am.Name(), edp.Attributes().AsRaw()))
			}
		}

		matchingDPS := make(map[pdata.NumberDataPoint]pdata.NumberDataPoint, actualDataPoints.Len())
		for j := 0; j < actualDataPoints.Len(); j++ {
			adp := actualDataPoints.At(j)
			var foundMatch bool
			for k := 0; k < expectedDataPoints.Len(); k++ {
				edp := expectedDataPoints.At(k)
				if reflect.DeepEqual(edp.Attributes().Sort(), adp.Attributes().Sort()) {
					foundMatch = true
					matchingDPS[adp] = edp
					break
				}
			}

			if !foundMatch {
				errs = multierr.Append(errs, fmt.Errorf("metric %s has extra data point: Labels: %v", am.Name(), adp.Attributes().AsRaw()))
			}
		}

		if errs != nil {
			return errs
		}

		for adp, edp := range matchingDPS {
			if edp.Type() != adp.Type() {
				return fmt.Errorf("metric datapoint types don't match: expected type: %v, actual type: %v", edp.Type(), adp.Type())
			}
			if edp.IntVal() != adp.IntVal() {
				return fmt.Errorf("metric datapoint IntVal doesn't match expected: %d, actual: %d", edp.IntVal(), adp.IntVal())
			}
			if edp.DoubleVal() != adp.DoubleVal() {
				return fmt.Errorf("metric datapoint DoubleVal doesn't match expected: %f, actual: %f", edp.DoubleVal(), adp.DoubleVal())
			}
		}
	}
	return nil
}
