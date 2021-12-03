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

package scrapertest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
)

// CompareOption is applied by the CompareMetricSlices function
// to mutates an expected and/or actual result before comparing.
type CompareOption interface {
	apply(expected, actual pdata.MetricSlice)
}

// CompareMetricSlices compares each part of two given MetricSlices and returns
// an error if they don't match. The error describes what didn't match. The
// expected and actual values are clones before options are applied.
func CompareMetricSlices(expected, actual pdata.MetricSlice, options ...CompareOption) error {
	expClone := pdata.NewMetricSlice()
	expected.CopyTo(expClone)
	actClone := pdata.NewMetricSlice()
	actual.CopyTo(actClone)
	expected, actual = expClone, actClone

	for _, option := range options {
		option.apply(expected, actual)
	}

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
	for name := range actualByName {
		_, ok := expectedByName[name]
		if !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected metric %s", name))
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

	for name, actualMetric := range actualByName {
		expectedMetric, ok := expectedByName[name]
		if !ok {
			return fmt.Errorf("metric name does not match expected: %s, actual: %s", expectedMetric.Name(), actualMetric.Name())
		}
		if actualMetric.Description() != expectedMetric.Description() {
			return fmt.Errorf("metric description does not match expected: %s, actual: %s", expectedMetric.Description(), actualMetric.Description())
		}
		if actualMetric.Unit() != expectedMetric.Unit() {
			return fmt.Errorf("metric Unit does not match expected: %s, actual: %s", expectedMetric.Unit(), actualMetric.Unit())
		}
		if actualMetric.DataType() != expectedMetric.DataType() {
			return fmt.Errorf("metric datatype does not match expected: %s, actual: %s", expectedMetric.DataType(), actualMetric.DataType())
		}

		var actualDataPoints pdata.NumberDataPointSlice
		var expectedDataPoints pdata.NumberDataPointSlice

		switch actualMetric.DataType() {
		case pdata.MetricDataTypeGauge:
			actualDataPoints = actualMetric.Gauge().DataPoints()
			expectedDataPoints = expectedMetric.Gauge().DataPoints()
		case pdata.MetricDataTypeSum:
			if actualMetric.Sum().AggregationTemporality() != expectedMetric.Sum().AggregationTemporality() {
				return fmt.Errorf("metric AggregationTemporality does not match expected: %s, actual: %s", expectedMetric.Sum().AggregationTemporality(), actualMetric.Sum().AggregationTemporality())
			}
			if actualMetric.Sum().IsMonotonic() != expectedMetric.Sum().IsMonotonic() {
				return fmt.Errorf("metric IsMonotonic does not match expected: %t, actual: %t", expectedMetric.Sum().IsMonotonic(), actualMetric.Sum().IsMonotonic())
			}
			actualDataPoints = actualMetric.Sum().DataPoints()
			expectedDataPoints = expectedMetric.Sum().DataPoints()
		}

		if err := CompareNumberDataPointSlices(actualDataPoints, expectedDataPoints); err != nil {
			return multierr.Combine(fmt.Errorf("datapoints for metric: `%s`, do not match expected", actualMetric.Name()), err)
		}
	}
	return nil
}

// CompareNumberDataPointSlices compares each part of two given NumberDataPointSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareNumberDataPointSlices(actual, expected pdata.NumberDataPointSlice) error {
	if actual.Len() != expected.Len() {
		return fmt.Errorf("length of datapoints don't match")
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

// CompareNumberDataPoints compares each part of two given NumberDataPoints and returns
// an error if they don't match. The error describes what didn't match.
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
