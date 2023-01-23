// Copyright The OpenTelemetry Authors
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

package pmetrictest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
)

func CompareMetrics(expected, actual pmetric.Metrics, options ...CompareMetricsOption) error {
	exp, act := pmetric.NewMetrics(), pmetric.NewMetrics()
	expected.CopyTo(exp)
	actual.CopyTo(act)

	for _, option := range options {
		option.applyOnMetrics(exp, act)
	}

	expectedMetrics, actualMetrics := exp.ResourceMetrics(), act.ResourceMetrics()
	if expectedMetrics.Len() != actualMetrics.Len() {
		return fmt.Errorf("number of resources does not match expected: %d, actual: %d", expectedMetrics.Len(),
			actualMetrics.Len())
	}

	numResources := expectedMetrics.Len()

	// Keep track of matching resources so that each can only be matched once
	matchingResources := make(map[pmetric.ResourceMetrics]pmetric.ResourceMetrics, numResources)

	var errs error
	var outOfOrderErrs error
	for e := 0; e < numResources; e++ {
		er := expectedMetrics.At(e)
		var foundMatch bool
		for a := 0; a < numResources; a++ {
			ar := actualMetrics.At(a)
			if _, ok := matchingResources[ar]; ok {
				continue
			}
			if reflect.DeepEqual(er.Resource().Attributes().AsRaw(), ar.Resource().Attributes().AsRaw()) {
				foundMatch = true
				matchingResources[ar] = er
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf("ResourceMetrics with attributes %v expected at index %d, "+
							"found a at index %d", er.Resource().Attributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected resource with attributes: %v", er.Resource().Attributes().AsRaw()))
		}
	}

	for i := 0; i < numResources; i++ {
		if _, ok := matchingResources[actualMetrics.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("extra resource with attributes: %v", actualMetrics.At(i).Resource().Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for ar, er := range matchingResources {
		if err := CompareResourceMetrics(er, ar); err != nil {
			return err
		}
	}

	return errs
}

func CompareResourceMetrics(expected, actual pmetric.ResourceMetrics) error {
	eilms := expected.ScopeMetrics()
	ailms := actual.ScopeMetrics()

	if eilms.Len() != ailms.Len() {
		return fmt.Errorf("number of instrumentation libraries does not match expected: %d, actual: %d", eilms.Len(),
			ailms.Len())
	}

	for i := 0; i < eilms.Len(); i++ {
		eilm, ailm := eilms.At(i), ailms.At(i)
		eil, ail := eilm.Scope(), ailm.Scope()

		if eil.Name() != ail.Name() {
			return fmt.Errorf("instrumentation library Name does not match expected: %s, actual: %s", eil.Name(), ail.Name())
		}
		if eil.Version() != ail.Version() {
			return fmt.Errorf("instrumentation library Version does not match expected: %s, actual: %s", eil.Version(), ail.Version())
		}

		if err := CompareMetricSlices(eilm.Metrics(), ailm.Metrics()); err != nil {
			return err
		}
	}
	return nil
}

// CompareMetricSlices compares each part of two given MetricSlices and returns
// an error if they don't match. The error describes what didn't match. The
// expected and actual values are clones before options are applied.
func CompareMetricSlices(expected, actual pmetric.MetricSlice) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("number of metrics does not match expected: %d, actual: %d", expected.Len(), actual.Len())
	}

	expectedByName, actualByName := metricsByName(expected), metricsByName(actual)

	var errs error
	for name := range actualByName {
		_, ok := expectedByName[name]
		if !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected metric: %s", name))
		}
	}
	for name := range expectedByName {
		if _, ok := actualByName[name]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("missing expected metric: %s", name))
		}
	}

	if errs != nil {
		return errs
	}

	for i := 0; i < actual.Len(); i++ {
		actualMetric := actual.At(i)
		expectedMetric := expected.At(i)
		if actualMetric.Name() != expectedMetric.Name() {
			return fmt.Errorf("metrics are out of order, metric %s expected at index %d, actual: %s",
				expectedMetric.Name(), i, actualMetric.Name())
		}
		if actualMetric.Description() != expectedMetric.Description() {
			return fmt.Errorf("metric Description does not match expected: %s, actual: %s", expectedMetric.Description(), actualMetric.Description())
		}
		if actualMetric.Unit() != expectedMetric.Unit() {
			return fmt.Errorf("metric Unit does not match expected: %s, actual: %s", expectedMetric.Unit(), actualMetric.Unit())
		}
		if actualMetric.Type() != expectedMetric.Type() {
			return fmt.Errorf("metric DataType does not match expected: %s, actual: %s", expectedMetric.Type(), actualMetric.Type())
		}

		switch actualMetric.Type() {
		case pmetric.MetricTypeGauge:
			if err := CompareNumberDataPointSlices(expectedMetric.Gauge().DataPoints(), actualMetric.Gauge().DataPoints()); err != nil {
				return multierr.Combine(fmt.Errorf("datapoints for metric: `%s`, do not match expected", actualMetric.Name()), err)
			}
		case pmetric.MetricTypeSum:
			if actualMetric.Sum().AggregationTemporality() != expectedMetric.Sum().AggregationTemporality() {
				return fmt.Errorf("metric AggregationTemporality does not match expected: %s, actual: %s", expectedMetric.Sum().AggregationTemporality(), actualMetric.Sum().AggregationTemporality())
			}
			if actualMetric.Sum().IsMonotonic() != expectedMetric.Sum().IsMonotonic() {
				return fmt.Errorf("metric IsMonotonic does not match expected: %t, actual: %t", expectedMetric.Sum().IsMonotonic(), actualMetric.Sum().IsMonotonic())
			}
			if err := CompareNumberDataPointSlices(expectedMetric.Sum().DataPoints(), actualMetric.Sum().DataPoints()); err != nil {
				return multierr.Combine(fmt.Errorf("datapoints for metric: `%s`, do not match expected", actualMetric.Name()), err)
			}
		case pmetric.MetricTypeHistogram:
			if actualMetric.Histogram().AggregationTemporality() != expectedMetric.Histogram().AggregationTemporality() {
				return fmt.Errorf("metric AggregationTemporality does not match expected: %s, actual: %s", expectedMetric.Histogram().AggregationTemporality(), actualMetric.Histogram().AggregationTemporality())
			}
			if err := CompareHistogramDataPointSlices(expectedMetric.Histogram().DataPoints(), actualMetric.Histogram().DataPoints()); err != nil {
				return multierr.Combine(fmt.Errorf("datapoints for metric: `%s`, do not match expected", actualMetric.Name()), err)
			}
		case pmetric.MetricTypeExponentialHistogram:
			if actualMetric.ExponentialHistogram().AggregationTemporality() != expectedMetric.ExponentialHistogram().AggregationTemporality() {
				return fmt.Errorf("metric AggregationTemporality does not match expected: %s, actual: %s", expectedMetric.ExponentialHistogram().AggregationTemporality(), actualMetric.ExponentialHistogram().AggregationTemporality())
			}
			if err := CompareExponentialHistogramDataPointSlices(expectedMetric.ExponentialHistogram().DataPoints(), actualMetric.ExponentialHistogram().DataPoints()); err != nil {
				return multierr.Combine(fmt.Errorf("datapoints for metric: `%s`, do not match expected", actualMetric.Name()), err)
			}
		case pmetric.MetricTypeSummary:
			if err := CompareSummaryDataPointSlices(expectedMetric.Summary().DataPoints(), actualMetric.Summary().DataPoints()); err != nil {
				return multierr.Combine(fmt.Errorf("datapoints for metric: `%s`, do not match expected", actualMetric.Name()), err)
			}
		}
	}
	return nil
}

// CompareNumberDataPointSlices compares each part of two given NumberDataPointSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareNumberDataPointSlices(expected, actual pmetric.NumberDataPointSlice) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("number of datapoints does not match expected: %d, actual: %d", expected.Len(), actual.Len())
	}

	numPoints := expected.Len()

	// Keep track of matching data points so that each point can only be matched once
	matchingDPS := make(map[pmetric.NumberDataPoint]pmetric.NumberDataPoint, numPoints)

	var errs error
	var outOfOrderErrs error
	for e := 0; e < numPoints; e++ {
		edp := expected.At(e)
		var foundMatch bool
		for a := 0; a < numPoints; a++ {
			adp := actual.At(a)
			if _, ok := matchingDPS[adp]; ok {
				continue
			}
			if reflect.DeepEqual(edp.Attributes().AsRaw(), adp.Attributes().AsRaw()) {
				foundMatch = true
				matchingDPS[adp] = edp
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs, fmt.Errorf("datapoints are out of order, "+
						"datapoint with attributes %v expected at index %d, "+
						"found a at index %d", edp.Attributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("metric missing expected datapoint with attributes: %v", edp.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numPoints; i++ {
		if _, ok := matchingDPS[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("metric has extra datapoint with attributes: %v", actual.At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for adp, edp := range matchingDPS {
		if err := CompareNumberDataPoints(edp, adp); err != nil {
			return multierr.Combine(fmt.Errorf("datapoint with attributes: %v, does not match expected", adp.Attributes().AsRaw()), err)
		}
	}
	return nil
}

// CompareNumberDataPoints compares each part of two given NumberDataPoints and returns
// an error if they don't match. The error describes what didn't match.
func CompareNumberDataPoints(expected, actual pmetric.NumberDataPoint) error {
	if expected.ValueType() != actual.ValueType() {
		return fmt.Errorf("metric datapoint types don't match: expected type: %s, actual type: %s", expected.ValueType(), actual.ValueType())
	}
	if expected.IntValue() != actual.IntValue() {
		return fmt.Errorf("metric datapoint IntVal doesn't match expected: %d, actual: %d", expected.IntValue(), actual.IntValue())
	}
	if expected.DoubleValue() != actual.DoubleValue() {
		return fmt.Errorf("metric datapoint DoubleVal doesn't match expected: %f, actual: %f", expected.DoubleValue(), actual.DoubleValue())
	}
	if err := CompareExemplars(expected.Exemplars(), actual.Exemplars()); err != nil {
		return multierr.Combine(fmt.Errorf("metric datapoint with exemplars: does not match expected"), err)
	}
	return nil
}

// CompareExemplars compares each part of two given ExemplarSlice and returns
// an error if they don't match. The error describes what didn't match.
func CompareExemplars(expected, actual pmetric.ExemplarSlice) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("number of exemplars does not match expected: %d, actual: %d", expected.Len(), actual.Len())
	}

	numExemplars := expected.Len()

	// Keep track of matching exemplars so that each exemplar can only be matched once
	matchingExs := make(map[pmetric.Exemplar]pmetric.Exemplar, numExemplars)

	var errs error
	var outOfOrderErrs error
	for e := 0; e < numExemplars; e++ {
		eex := expected.At(e)
		var foundMatch bool
		for a := 0; a < numExemplars; a++ {
			aex := actual.At(a)
			if _, ok := matchingExs[aex]; ok {
				continue
			}
			if reflect.DeepEqual(eex.FilteredAttributes().AsRaw(), aex.FilteredAttributes().AsRaw()) {
				foundMatch = true
				matchingExs[aex] = eex
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs, fmt.Errorf("exemplars are out of order, "+
						"exemplar with attributes %v expected at index %d, "+
						"found a at index %d", eex.FilteredAttributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("exemplars missing expected exemplar with filtered attributes: %v", eex.FilteredAttributes().AsRaw()))
		}
	}

	for i := 0; i < numExemplars; i++ {
		if _, ok := matchingExs[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("exemplars has extra exemplar with attributes: %v", actual.At(i).FilteredAttributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for aex, eex := range matchingExs {
		if err := CompareExemplar(eex, aex); err != nil {
			return multierr.Combine(fmt.Errorf("exemplar with filtered attributes: %v, does not match expected", aex.FilteredAttributes().AsRaw()), err)
		}
	}
	return nil
}

// CompareExemplars compares each part of two given pmetric.Exemplar and returns
// an error if they don't match. The error describes what didn't match.
func CompareExemplar(expected, actual pmetric.Exemplar) error {
	if expected.ValueType() != actual.ValueType() {
		return fmt.Errorf("exemplar types don't match: expected type: %s, actual type: %s", expected.ValueType(), actual.ValueType())
	}
	if expected.DoubleValue() != actual.DoubleValue() {
		return fmt.Errorf("exemplar DoubleVal doesn't match expected: %f, actual: %f", expected.DoubleValue(), actual.DoubleValue())
	}
	if expected.IntValue() != actual.IntValue() {
		return fmt.Errorf("exemplar IntValue doesn't match expected: %d, actual: %d", expected.IntValue(), actual.IntValue())
	}
	if expected.Timestamp() != actual.Timestamp() {
		return fmt.Errorf("exemplar timestamp doesn't match expected: %d, actual: %d", expected.Timestamp(), actual.Timestamp())
	}
	if expected.TraceID() != actual.TraceID() {
		return fmt.Errorf("exemplar traceID doesn't match expected: %s, actual: %s", expected.TraceID(), actual.TraceID())
	}
	if expected.SpanID() != actual.SpanID() {
		return fmt.Errorf("exemplar spanID doesn't match expected: %s, actual: %s", expected.SpanID(), actual.SpanID())
	}
	return nil
}

// CompareHistogramDataPointSlices compares each part of two given HistogramDataPointSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareHistogramDataPointSlices(expected, actual pmetric.HistogramDataPointSlice) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("number of datapoints does not match expected: %d, actual: %d", expected.Len(), actual.Len())
	}

	numPoints := expected.Len()

	// Keep track of matching data points so that each point can only be matched once
	matchingDPS := make(map[pmetric.HistogramDataPoint]pmetric.HistogramDataPoint, numPoints)

	var errs error
	var outOfOrderErrs error
	for e := 0; e < numPoints; e++ {
		edp := expected.At(e)
		var foundMatch bool
		for a := 0; a < numPoints; a++ {
			adp := actual.At(a)
			if _, ok := matchingDPS[adp]; ok {
				continue
			}
			if reflect.DeepEqual(edp.Attributes().AsRaw(), adp.Attributes().AsRaw()) {
				foundMatch = true
				matchingDPS[adp] = edp
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf("datapoint with attributes %v expected at index %d, "+
							"found a at index %d", edp.Attributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("metric missing expected datapoint with attributes: %v", edp.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numPoints; i++ {
		if _, ok := matchingDPS[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("metric has extra datapoint with attributes: %v", actual.At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for adp, edp := range matchingDPS {
		if err := CompareHistogramDataPoints(edp, adp); err != nil {
			return multierr.Combine(fmt.Errorf("datapoint with attributes: %v, does not match expected", adp.Attributes().AsRaw()), err)
		}
	}
	return nil
}

// CompareHistogramDataPoints compares each part of two given HistogramDataPoints and returns
// an error if they don't match. The error describes what didn't match.
func CompareHistogramDataPoints(expected, actual pmetric.HistogramDataPoint) error {
	if expected.HasSum() != actual.HasSum() {
		return fmt.Errorf("metric datapoint HasSum doesn't match expected: %t, actual: %t", expected.HasSum(), actual.HasSum())
	}
	if expected.HasSum() && expected.Sum() != actual.Sum() {
		return fmt.Errorf("metric datapoint Sum doesn't match expected: %f, actual: %f", expected.Sum(), actual.Sum())
	}
	if expected.HasMin() != actual.HasMin() {
		return fmt.Errorf("metric datapoint HasMin doesn't match expected: %t, actual: %t", expected.HasMin(), actual.HasMin())
	}
	if expected.HasMin() && expected.Min() != actual.Min() {
		return fmt.Errorf("metric datapoint Min doesn't match expected: %f, actual: %f", expected.Min(), actual.Min())
	}
	if expected.HasMax() != actual.HasMax() {
		return fmt.Errorf("metric datapoint HasMax doesn't match expected: %t, actual: %t", expected.HasMax(), actual.HasMax())
	}
	if expected.HasMax() && expected.Max() != actual.Max() {
		return fmt.Errorf("metric datapoint Max doesn't match expected: %f, actual: %f", expected.Max(), actual.Max())
	}
	if expected.Count() != actual.Count() {
		return fmt.Errorf("metric datapoint Count doesn't match expected: %d, actual: %d", expected.Count(), actual.Count())
	}
	if expected.StartTimestamp() != actual.StartTimestamp() {
		return fmt.Errorf("metric datapoint StartTimestamp doesn't match expected: %d, actual: %d", expected.StartTimestamp(), actual.StartTimestamp())
	}
	if expected.Timestamp() != actual.Timestamp() {
		return fmt.Errorf("metric datapoint Timestamp doesn't match expected: %d, actual: %d", expected.Timestamp(), actual.Timestamp())
	}
	if expected.Flags() != actual.Flags() {
		return fmt.Errorf("metric datapoint Flags doesn't match expected: %d, actual: %d", expected.Flags(), actual.Flags())
	}
	if !reflect.DeepEqual(expected.BucketCounts(), actual.BucketCounts()) {
		return fmt.Errorf("metric datapoint BucketCounts doesn't match expected: %v, actual: %v", expected.BucketCounts().AsRaw(), actual.BucketCounts().AsRaw())
	}
	if !reflect.DeepEqual(expected.ExplicitBounds(), actual.ExplicitBounds()) {
		return fmt.Errorf("metric datapoint ExplicitBounds doesn't match expected: %v, actual: %v", expected.ExplicitBounds().AsRaw(), actual.ExplicitBounds().AsRaw())
	}
	if !reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) {
		return fmt.Errorf("metric datapoint Attributes doesn't match expected: %v, actual: %v", expected.Attributes().AsRaw(), actual.Attributes().AsRaw())
	}
	if err := CompareExemplars(expected.Exemplars(), actual.Exemplars()); err != nil {
		return multierr.Combine(fmt.Errorf("metric datapoint with exemplars: does not match expected"), err)
	}
	return nil
}

// CompareExponentialHistogramDataPointSlices compares each part of two given ExponentialHistogramDataPointSlices and returns
// an error if they don't match. The error describes what didn't match.
func CompareExponentialHistogramDataPointSlices(expected, actual pmetric.ExponentialHistogramDataPointSlice) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("number of datapoints does not match expected: %d, actual: %d", expected.Len(), actual.Len())
	}

	numPoints := expected.Len()

	// Keep track of matching data points so that each point can only be matched once
	matchingDPS := make(map[pmetric.ExponentialHistogramDataPoint]pmetric.ExponentialHistogramDataPoint, numPoints)

	var errs error
	var outOfOrderErrs error
	for e := 0; e < numPoints; e++ {
		edp := expected.At(e)
		var foundMatch bool
		for a := 0; a < numPoints; a++ {
			adp := actual.At(a)
			if _, ok := matchingDPS[adp]; ok {
				continue
			}
			if reflect.DeepEqual(edp.Attributes().AsRaw(), adp.Attributes().AsRaw()) {
				foundMatch = true
				matchingDPS[adp] = edp
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf("datapoint with attributes %v expected at index %d, "+
							"found a at index %d", edp.Attributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("metric missing expected datapoint with attributes: %v", edp.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numPoints; i++ {
		if _, ok := matchingDPS[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("metric has extra datapoint with attributes: %v", actual.At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for adp, edp := range matchingDPS {
		if err := CompareExponentialHistogramDataPoints(edp, adp); err != nil {
			return multierr.Combine(fmt.Errorf("datapoint with attributes: %v, does not match expected", adp.Attributes().AsRaw()), err)
		}
	}
	return nil
}

// CompareExponentialHistogramDataPoints compares each part of two given ExponentialHistogramDataPoints and returns
// an error if they don't match. The error describes what didn't match.
func CompareExponentialHistogramDataPoints(expected, actual pmetric.ExponentialHistogramDataPoint) error {
	if expected.HasSum() != actual.HasSum() {
		return fmt.Errorf("metric datapoint HasSum doesn't match expected: %t, actual: %t", expected.HasSum(), actual.HasSum())
	}
	if expected.HasSum() && expected.Sum() != actual.Sum() {
		return fmt.Errorf("metric datapoint Sum doesn't match expected: %f, actual: %f", expected.Sum(), actual.Sum())
	}
	if expected.HasMin() != actual.HasMin() {
		return fmt.Errorf("metric datapoint HasMin doesn't match expected: %t, actual: %t", expected.HasMin(), actual.HasMin())
	}
	if expected.HasMin() && expected.Min() != actual.Min() {
		return fmt.Errorf("metric datapoint Min doesn't match expected: %f, actual: %f", expected.Min(), actual.Min())
	}
	if expected.HasMax() != actual.HasMax() {
		return fmt.Errorf("metric datapoint HasMax doesn't match expected: %t, actual: %t", expected.HasMax(), actual.HasMax())
	}
	if expected.HasMax() && expected.Max() != actual.Max() {
		return fmt.Errorf("metric datapoint Max doesn't match expected: %f, actual: %f", expected.Max(), actual.Max())
	}
	if expected.Count() != actual.Count() {
		return fmt.Errorf("metric datapoint Count doesn't match expected: %d, actual: %d", expected.Count(), actual.Count())
	}
	if expected.ZeroCount() != actual.ZeroCount() {
		return fmt.Errorf("metric datapoint ZeroCount doesn't match expected: %d, actual: %d", expected.ZeroCount(), actual.ZeroCount())
	}
	if expected.StartTimestamp() != actual.StartTimestamp() {
		return fmt.Errorf("metric datapoint StartTimestamp doesn't match expected: %d, actual: %d", expected.StartTimestamp(), actual.StartTimestamp())
	}
	if expected.Timestamp() != actual.Timestamp() {
		return fmt.Errorf("metric datapoint Timestamp doesn't match expected: %d, actual: %d", expected.Timestamp(), actual.Timestamp())
	}
	if expected.Flags() != actual.Flags() {
		return fmt.Errorf("metric datapoint Flags doesn't match expected: %d, actual: %d", expected.Flags(), actual.Flags())
	}
	if expected.Scale() != actual.Scale() {
		return fmt.Errorf("metric datapoint Scale doesn't match expected: %v, actual: %v", expected.Scale(), actual.Scale())
	}
	if expected.Negative().Offset() != actual.Negative().Offset() {
		return fmt.Errorf("metric datapoint Negative Offset doesn't match expected: %v, actual: %v", expected.Negative().Offset(), actual.Negative().Offset())
	}
	if !reflect.DeepEqual(expected.Negative().BucketCounts(), actual.Negative().BucketCounts()) {
		return fmt.Errorf("metric datapoint Negative BucketCounts doesn't match expected: %v, actual: %v",
			expected.Negative().BucketCounts().AsRaw(), actual.Negative().BucketCounts().AsRaw())
	}
	if expected.Positive().Offset() != actual.Positive().Offset() {
		return fmt.Errorf("metric datapoint Positive Offset doesn't match expected: %v, actual: %v", expected.Positive().Offset(), actual.Positive().Offset())
	}
	if !reflect.DeepEqual(expected.Positive().BucketCounts(), actual.Positive().BucketCounts()) {
		return fmt.Errorf("metric datapoint Positive BucketCounts doesn't match expected: %v, actual: %v",
			expected.Positive().BucketCounts().AsRaw(), actual.Positive().BucketCounts().AsRaw())
	}
	if !reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) {
		return fmt.Errorf("metric datapoint Attributes doesn't match expected: %v, actual: %v", expected.Attributes().AsRaw(), actual.Attributes().AsRaw())
	}
	if err := CompareExemplars(expected.Exemplars(), actual.Exemplars()); err != nil {
		return multierr.Combine(fmt.Errorf("metric datapoint with exemplars: does not match expected"), err)
	}
	return nil
}

// CompareSummaryDataPointSlices compares each part of two given SummaryDataPoint slices and returns
// an error if they don't match. The error describes what didn't match.
func CompareSummaryDataPointSlices(expected, actual pmetric.SummaryDataPointSlice) error {
	numPoints := expected.Len()
	if numPoints != actual.Len() {
		return fmt.Errorf("metric datapoint slice length doesn't match expected: %d, actual: %d", numPoints, actual.Len())
	}

	matchingDPS := map[pmetric.SummaryDataPoint]pmetric.SummaryDataPoint{}
	var errs error
	var outOfOrderErrs error
	for e := 0; e < numPoints; e++ {
		edp := expected.At(e)
		var foundMatch bool
		for a := 0; a < numPoints; a++ {
			adp := actual.At(a)
			if _, ok := matchingDPS[adp]; ok {
				continue
			}
			if reflect.DeepEqual(edp.Attributes().AsRaw(), adp.Attributes().AsRaw()) {
				foundMatch = true
				matchingDPS[adp] = edp
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf("datapoint with attributes %v expected at index %d, "+
							"found a at index %d", edp.Attributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("metric missing expected datapoint with attributes: %v", edp.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numPoints; i++ {
		if _, ok := matchingDPS[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("metric has extra datapoint with attributes: %v", actual.At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for adp, edp := range matchingDPS {
		if err := CompareSummaryDataPoints(edp, adp); err != nil {
			return multierr.Combine(fmt.Errorf("datapoint with attributes: %v, does not match expected", adp.Attributes().AsRaw()), err)
		}
	}
	return nil
}

// CompareSummaryDataPoints compares each part of two given SummaryDataPoint and returns
// an error if they don't match. The error describes what didn't match.
func CompareSummaryDataPoints(expected, actual pmetric.SummaryDataPoint) error {
	if expected.Count() != actual.Count() {
		return fmt.Errorf("metric datapoint Count doesn't match expected: %d, actual: %d", expected.Count(), actual.Count())
	}
	if expected.Sum() != actual.Sum() {
		return fmt.Errorf("metric datapoint Sum doesn't match expected: %f, actual: %f", expected.Sum(), actual.Sum())
	}
	if expected.StartTimestamp() != actual.StartTimestamp() {
		return fmt.Errorf("metric datapoint StartTimestamp doesn't match expected: %d, actual: %d", expected.StartTimestamp(), actual.StartTimestamp())
	}
	if expected.Timestamp() != actual.Timestamp() {
		return fmt.Errorf("metric datapoint Timestamp doesn't match expected: %d, actual: %d", expected.Timestamp(), actual.Timestamp())
	}
	if expected.Flags() != actual.Flags() {
		return fmt.Errorf("metric datapoint Flags doesn't match expected: %d, actual: %d", expected.Flags(), actual.Flags())
	}
	if !reflect.DeepEqual(expected.Attributes().AsRaw(), actual.Attributes().AsRaw()) {
		return fmt.Errorf("metric datapoint Attributes doesn't match expected: %v, actual: %v", expected.Attributes().AsRaw(), actual.Attributes().AsRaw())
	}
	if expected.QuantileValues().Len() != actual.QuantileValues().Len() {
		return fmt.Errorf("metric datapoint QuantileValues length doesn't match expected: %d, actual: %d", expected.QuantileValues().Len(), actual.QuantileValues().Len())
	}

	for i := 0; i < expected.QuantileValues().Len(); i++ {
		eqv, acv := expected.QuantileValues().At(i), actual.QuantileValues().At(i)
		if eqv.Quantile() != acv.Quantile() {
			return fmt.Errorf("metric datapoint quantile doesn't match expected: %f, actual: %f", eqv.Quantile(), acv.Quantile())
		}
		if eqv.Value() != acv.Value() {
			return fmt.Errorf("metric datapoint value at quantile %f doesn't match expected: %f, actual: %f",
				eqv.Quantile(), eqv.Value(), acv.Value())
		}
	}

	return nil
}
