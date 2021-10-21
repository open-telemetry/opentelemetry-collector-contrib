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

		if err := CompareNumberDataPointSlices(actualDataPoints, expectedDataPoints); err != nil {
			return multierr.Combine(fmt.Errorf("datapoints for metric: `%s`, do not match expected", am.Name()), err)
		}
	}
	return nil
}

func CompareNumberDataPointSlices(actual, expected pdata.NumberDataPointSlice) error {
	if actual.Len() != expected.Len() {
		return fmt.Errorf("length of datapoints don't match, metric name")
	}

	var errs error
	for j := 0; j < expected.Len(); j++ {
		edp := expected.At(j)
		var foundMatch bool
		for k := 0; k < actual.Len(); k++ {
			adp := actual.At(k)
			if reflect.DeepEqual(adp.Attributes().Sort().AsRaw(), edp.Attributes().Sort().AsRaw()) {
				foundMatch = true
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("metric missing expected data point: Labels: %v", edp.Attributes().AsRaw()))
		}
	}

	matchingDPS := make(map[pdata.NumberDataPoint]pdata.NumberDataPoint, actual.Len())
	for j := 0; j < actual.Len(); j++ {
		adp := actual.At(j)
		var foundMatch bool
		for k := 0; k < expected.Len(); k++ {
			edp := expected.At(k)
			if reflect.DeepEqual(edp.Attributes().Sort(), adp.Attributes().Sort()) {
				foundMatch = true
				matchingDPS[adp] = edp
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("metric has extra data point: Labels: %v", adp.Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}

	for adp, edp := range matchingDPS {
		if err := CompareNumberDataPoints(adp, edp); err != nil {
			return multierr.Combine(fmt.Errorf("datapoint with label(s): %v, does not match expected", adp.Attributes().AsRaw()), err)
		}
	}
	return nil
}

func CompareNumberDataPoints(actual, expected pdata.NumberDataPoint) error {
	if expected.Type() != actual.Type() {
		return fmt.Errorf("metric datapoint types don't match: expected type: %v, actual type: %v", expected.Type(), actual.Type())
	}
	if expected.IntVal() != actual.IntVal() {
		return fmt.Errorf("metric datapoint IntVal doesn't match expected: %d, actual: %d", expected.IntVal(), actual.IntVal())
	}
	if expected.DoubleVal() != actual.DoubleVal() {
		return fmt.Errorf("metric datapoint DoubleVal doesn't match expected: %f, actual: %f", expected.DoubleVal(), actual.DoubleVal())
	}
	return nil
}
