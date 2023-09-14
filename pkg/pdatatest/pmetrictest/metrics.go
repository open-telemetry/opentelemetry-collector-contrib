// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetrictest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"

import (
	"errors"
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
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
		return fmt.Errorf("number of resources doesn't match expected: %d, actual: %d", expectedMetrics.Len(),
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
						fmt.Errorf(`resources are out of order: resource "%v" expected at index %d, found at index %d`,
							er.Resource().Attributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected resource: %v", er.Resource().Attributes().AsRaw()))
		}
	}

	for i := 0; i < numResources; i++ {
		if _, ok := matchingResources[actualMetrics.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected resource: %v",
				actualMetrics.At(i).Resource().Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for ar, er := range matchingResources {
		errPrefix := fmt.Sprintf(`resource "%v"`, ar.Resource().Attributes().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareResourceMetrics(er, ar)))
	}

	return errs
}

func CompareResourceMetrics(expected, actual pmetric.ResourceMetrics) error {
	errs := multierr.Combine(
		internal.CompareResource(expected.Resource(), actual.Resource()),
		internal.CompareSchemaURL(expected.SchemaUrl(), actual.SchemaUrl()),
	)

	esms := expected.ScopeMetrics()
	asms := actual.ScopeMetrics()

	if esms.Len() != asms.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of scopes doesn't match expected: %d, actual: %d", esms.Len(),
			asms.Len()))
		return errs
	}

	numScopeMetrics := expected.ScopeMetrics().Len()

	// Keep track of matching resources so that each can only be matched once
	matchingResources := make(map[pmetric.ScopeMetrics]pmetric.ScopeMetrics, numScopeMetrics)

	var outOfOrderErrs error
	for e := 0; e < numScopeMetrics; e++ {
		esm := esms.At(e)
		var foundMatch bool
		for a := 0; a < numScopeMetrics; a++ {
			asm := asms.At(a)
			if _, ok := matchingResources[asm]; ok {
				continue
			}
			if esm.Scope().Name() == asm.Scope().Name() {
				foundMatch = true
				matchingResources[asm] = esm
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`scopes are out of order: scope "%s" expected at index %d, found at index %d`,
							esm.Scope().Name(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected scope: %s", esm.Scope().Name()))
		}
	}

	for i := 0; i < numScopeMetrics; i++ {
		if _, ok := matchingResources[actual.ScopeMetrics().At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected scope: %s",
				actual.ScopeMetrics().At(i).Scope().Name()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for i := 0; i < esms.Len(); i++ {
		errPrefix := fmt.Sprintf(`scope "%s"`, esms.At(i).Scope().Name())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareScopeMetrics(esms.At(i), asms.At(i))))
	}

	return errs
}

// CompareScopeMetrics compares each part of two given ScopeMetrics and returns
// an error if they don't match. The error describes what didn't match. The
// expected and actual values are clones before options are applied.
func CompareScopeMetrics(expected, actual pmetric.ScopeMetrics) error {
	errs := multierr.Combine(
		internal.CompareInstrumentationScope(expected.Scope(), actual.Scope()),
		internal.CompareSchemaURL(expected.SchemaUrl(), actual.SchemaUrl()),
	)

	ems := expected.Metrics()
	ams := actual.Metrics()

	if ems.Len() != ams.Len() {
		errs = multierr.Append(errs, fmt.Errorf("number of metrics doesn't match expected: %d, actual: %d",
			ems.Len(), ams.Len()))
		return errs
	}

	numMetrics := ems.Len()

	// Keep track of matching records so that each record can only be matched once
	matchingMetrics := make(map[pmetric.Metric]pmetric.Metric, numMetrics)

	var outOfOrderErrs error
	for e := 0; e < numMetrics; e++ {
		em := ems.At(e)
		var foundMatch bool
		for a := 0; a < numMetrics; a++ {
			am := ams.At(a)
			if _, ok := matchingMetrics[am]; ok {
				continue
			}
			if em.Name() == am.Name() {
				foundMatch = true
				matchingMetrics[am] = em
				if e != a {
					outOfOrderErrs = multierr.Append(outOfOrderErrs,
						fmt.Errorf(`metrics are out of order: metric "%s" expected at index %d, found at index %d`,
							em.Name(), e, a))
				}
				break
			}
		}
		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected metric: %s", em.Name()))
		}
	}

	for i := 0; i < numMetrics; i++ {
		if _, ok := matchingMetrics[ams.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected metric: %s", ams.At(i).Name()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for i := 0; i < numMetrics; i++ {
		errPrefix := fmt.Sprintf(`metric "%s"`, ems.At(i).Name())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareMetric(ems.At(i), ams.At(i))))
	}
	return errs
}

func CompareMetric(expected pmetric.Metric, actual pmetric.Metric) error {
	var errs error

	if actual.Name() != expected.Name() {
		errs = multierr.Append(errs, fmt.Errorf("name doesn't match expected: %s, actual: %s", expected.Name(),
			actual.Name()))
	}
	if actual.Description() != expected.Description() {
		errs = multierr.Append(errs, fmt.Errorf("description doesn't match expected: %s, actual: %s",
			expected.Description(), actual.Description()))
	}
	if actual.Unit() != expected.Unit() {
		errs = multierr.Append(errs, fmt.Errorf("unit doesn't match expected: %s, actual: %s", expected.Unit(),
			actual.Unit()))
	}
	if actual.Type() != expected.Type() {
		errs = multierr.Append(errs, fmt.Errorf("type doesn't match expected: %s, actual: %s", expected.Type(),
			actual.Type()))
		return errs
	}

	//exhaustive:enforce
	switch actual.Type() {
	case pmetric.MetricTypeGauge:
		errs = multierr.Append(errs, compareNumberDataPointSlices(expected.Gauge().DataPoints(),
			actual.Gauge().DataPoints()))
	case pmetric.MetricTypeSum:
		if actual.Sum().AggregationTemporality() != expected.Sum().AggregationTemporality() {
			errs = multierr.Append(errs, fmt.Errorf("aggregation temporality doesn't match expected: %s, actual: %s",
				expected.Sum().AggregationTemporality(), actual.Sum().AggregationTemporality()))
		}
		if actual.Sum().IsMonotonic() != expected.Sum().IsMonotonic() {
			errs = multierr.Append(errs, fmt.Errorf("is monotonic doesn't match expected: %t, actual: %t",
				expected.Sum().IsMonotonic(), actual.Sum().IsMonotonic()))
		}
		errs = multierr.Append(errs, compareNumberDataPointSlices(expected.Sum().DataPoints(), actual.Sum().DataPoints()))
	case pmetric.MetricTypeHistogram:
		if actual.Histogram().AggregationTemporality() != expected.Histogram().AggregationTemporality() {
			errs = multierr.Append(errs, fmt.Errorf("aggregation temporality doesn't match expected: %s, actual: %s",
				expected.Histogram().AggregationTemporality(), actual.Histogram().AggregationTemporality()))
		}
		errs = multierr.Append(errs, compareHistogramDataPointSlices(expected.Histogram().DataPoints(),
			actual.Histogram().DataPoints()))
	case pmetric.MetricTypeExponentialHistogram:
		if actual.ExponentialHistogram().AggregationTemporality() != expected.ExponentialHistogram().AggregationTemporality() {
			errs = multierr.Append(errs, fmt.Errorf("aggregation temporality doesn't match expected: %s, actual: %s",
				expected.ExponentialHistogram().AggregationTemporality(),
				actual.ExponentialHistogram().AggregationTemporality()))
		}
		errs = multierr.Append(errs, compareExponentialHistogramDataPointSlice(expected.ExponentialHistogram().DataPoints(),
			actual.ExponentialHistogram().DataPoints()))
	case pmetric.MetricTypeSummary:
		errs = multierr.Append(errs, compareSummaryDataPointSlices(expected.Summary().DataPoints(),
			actual.Summary().DataPoints()))
	case pmetric.MetricTypeEmpty:
	}

	return errs
}

// compareNumberDataPointSlices compares each part of two given NumberDataPointSlices and returns
// an error if they don't match. The error describes what didn't match.
func compareNumberDataPointSlices(expected, actual pmetric.NumberDataPointSlice) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("number of datapoints doesn't match expected: %d, actual: %d", expected.Len(), actual.Len())
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
					outOfOrderErrs = multierr.Append(outOfOrderErrs, fmt.Errorf("datapoints are out of order: "+
						`datapoint "%v" expected at index %d, found at index %d`, edp.Attributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected datapoint: %v", edp.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numPoints; i++ {
		if _, ok := matchingDPS[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected datapoint: %v", actual.At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for adp, edp := range matchingDPS {
		errPrefix := fmt.Sprintf(`datapoint "%v"`, adp.Attributes().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareNumberDataPoint(edp, adp)))
	}

	return errs
}

// CompareNumberDataPoint compares each part of two given NumberDataPoints and returns
// an error if they don't match. The error describes what didn't match.
func CompareNumberDataPoint(expected, actual pmetric.NumberDataPoint) error {
	errs := internal.CompareAttributes(expected.Attributes(), actual.Attributes())
	if expected.StartTimestamp() != actual.StartTimestamp() {
		errs = multierr.Append(errs, fmt.Errorf("start timestamp doesn't match expected: %d, "+
			"actual: %d", expected.StartTimestamp(), actual.StartTimestamp()))
	}
	if expected.Timestamp() != actual.Timestamp() {
		errs = multierr.Append(errs, fmt.Errorf("timestamp doesn't match expected: %d, actual: %d",
			expected.Timestamp(), actual.Timestamp()))
	}
	if expected.ValueType() != actual.ValueType() {
		errs = multierr.Append(errs, fmt.Errorf("value type doesn't match expected: %s, actual: %s",
			expected.ValueType(), actual.ValueType()))
	} else {
		if expected.IntValue() != actual.IntValue() {
			errs = multierr.Append(errs, fmt.Errorf("int value doesn't match expected: %d, actual: %d",
				expected.IntValue(), actual.IntValue()))
		}
		if expected.DoubleValue() != actual.DoubleValue() {
			errs = multierr.Append(errs, fmt.Errorf("double value doesn't match expected: %f, actual: %f",
				expected.DoubleValue(), actual.DoubleValue()))
		}
	}
	if expected.Flags() != actual.Flags() {
		errs = multierr.Append(errs, fmt.Errorf("flags don't match expected: %d, actual: %d",
			expected.Flags(), actual.Flags()))
	}
	errs = multierr.Append(errs, compareExemplarSlice(expected.Exemplars(), actual.Exemplars()))
	return errs
}

// compareExemplarSlice compares each part of two given ExemplarSlice and returns
// an error if they don't match. The error describes what didn't match.
func compareExemplarSlice(expected, actual pmetric.ExemplarSlice) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("number of exemplars doesn't match expected: %d, actual: %d", expected.Len(), actual.Len())
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
					outOfOrderErrs = multierr.Append(outOfOrderErrs, fmt.Errorf("exemplars are out of order: "+
						"exemplar %v expected at index %d, found at index %d", eex.FilteredAttributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected exemplar: %v", eex.FilteredAttributes().AsRaw()))
		}
	}

	for i := 0; i < numExemplars; i++ {
		if _, ok := matchingExs[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected exemplar: %v",
				actual.At(i).FilteredAttributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for aex, eex := range matchingExs {
		errPrefix := fmt.Sprintf(`exemplar %v`, eex.FilteredAttributes().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareExemplar(eex, aex)))
	}
	return errs
}

// CompareExemplar compares each part of two given pmetric.Exemplar and returns
// an error if they don't match. The error describes what didn't match.
func CompareExemplar(expected, actual pmetric.Exemplar) error {
	var errs error
	if expected.ValueType() != actual.ValueType() {
		errs = multierr.Append(errs, fmt.Errorf("value type doesn't match: expected type: %s, actual type: %s",
			expected.ValueType(), actual.ValueType()))
	} else {
		if expected.DoubleValue() != actual.DoubleValue() {
			errs = multierr.Append(errs, fmt.Errorf("double value doesn't match expected: %f, actual: %f",
				expected.DoubleValue(), actual.DoubleValue()))
		}
		if expected.IntValue() != actual.IntValue() {
			errs = multierr.Append(errs, fmt.Errorf("int value doesn't match expected: %d, actual: %d",
				expected.IntValue(), actual.IntValue()))
		}
	}
	if expected.Timestamp() != actual.Timestamp() {
		errs = multierr.Append(errs, fmt.Errorf("timestamp doesn't match expected: %d, actual: %d",
			expected.Timestamp(), actual.Timestamp()))
	}
	if expected.TraceID() != actual.TraceID() {
		errs = multierr.Append(errs, fmt.Errorf("trace ID doesn't match expected: %s, actual: %s",
			expected.TraceID(), actual.TraceID()))
	}
	if expected.SpanID() != actual.SpanID() {
		errs = multierr.Append(errs, fmt.Errorf("span ID doesn't match expected: %s, actual: %s",
			expected.SpanID(), actual.SpanID()))
	}
	return errs
}

// compareHistogramDataPointSlices compares each part of two given HistogramDataPointSlices and returns
// an error if they don't match. The error describes what didn't match.
func compareHistogramDataPointSlices(expected, actual pmetric.HistogramDataPointSlice) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("number of datapoints doesn't match expected: %d, actual: %d", expected.Len(), actual.Len())
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
						fmt.Errorf(`datapoint "%v" expected at index %d, found at index %d`, edp.Attributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected datapoint: %v", edp.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numPoints; i++ {
		if _, ok := matchingDPS[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected datapoint: %v", actual.At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for adp, edp := range matchingDPS {
		errPrefix := fmt.Sprintf(`datapoint "%v"`, adp.Attributes().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareHistogramDataPoints(edp, adp)))
	}
	return errs
}

// CompareHistogramDataPoints compares each part of two given HistogramDataPoints and returns
// an error if they don't match. The error describes what didn't match.
func CompareHistogramDataPoints(expected, actual pmetric.HistogramDataPoint) error {
	errs := internal.CompareAttributes(expected.Attributes(), actual.Attributes())
	if expected.HasSum() != actual.HasSum() || expected.Sum() != actual.Sum() {
		errStr := "sum doesn't match expected: "
		if expected.HasSum() {
			errStr += fmt.Sprintf("%f", expected.Sum())
		} else {
			errStr += "<unset>"
		}
		errStr += ", actual: "
		if actual.HasSum() {
			errStr += fmt.Sprintf("%f", actual.Sum())
		} else {
			errStr += "<unset>"
		}
		errs = multierr.Append(errs, errors.New(errStr))
	}
	if expected.HasMin() != actual.HasMin() || expected.Min() != actual.Min() {
		errStr := "min doesn't match expected: "
		if expected.HasMin() {
			errStr += fmt.Sprintf("%f", expected.Min())
		} else {
			errStr += "<unset>"
		}
		errStr += ", actual: "
		if actual.HasMin() {
			errStr += fmt.Sprintf("%f", actual.Min())
		} else {
			errStr += "<unset>"
		}
		errs = multierr.Append(errs, errors.New(errStr))
	}
	if expected.HasMax() != actual.HasMax() || expected.Max() != actual.Max() {
		errStr := "max doesn't match expected: "
		if expected.HasMax() {
			errStr += fmt.Sprintf("%f", expected.Min())
		} else {
			errStr += "<unset>"
		}
		errStr += ", actual: "
		if actual.HasMax() {
			errStr += fmt.Sprintf("%f", actual.Min())
		} else {
			errStr += "<unset>"
		}
		errs = multierr.Append(errs, errors.New(errStr))
	}
	if expected.Count() != actual.Count() {
		errs = multierr.Append(errs, fmt.Errorf("count doesn't match expected: %d, actual: %d",
			expected.Count(), actual.Count()))
	}
	if expected.StartTimestamp() != actual.StartTimestamp() {
		errs = multierr.Append(errs, fmt.Errorf("start timestamp doesn't match expected: %d, actual: %d",
			expected.StartTimestamp(), actual.StartTimestamp()))
	}
	if expected.Timestamp() != actual.Timestamp() {
		errs = multierr.Append(errs, fmt.Errorf("timestamp doesn't match expected: %d, actual: %d",
			expected.Timestamp(), actual.Timestamp()))
	}
	if expected.Flags() != actual.Flags() {
		errs = multierr.Append(errs, fmt.Errorf("flags don't match expected: %d, actual: %d",
			expected.Flags(), actual.Flags()))
	}
	if !reflect.DeepEqual(expected.BucketCounts(), actual.BucketCounts()) {
		errs = multierr.Append(errs, fmt.Errorf("bucket counts don't match expected: %v, actual: %v",
			expected.BucketCounts().AsRaw(), actual.BucketCounts().AsRaw()))
	}
	if !reflect.DeepEqual(expected.ExplicitBounds(), actual.ExplicitBounds()) {
		errs = multierr.Append(errs, fmt.Errorf("explicit bounds don't match expected: %v, "+
			"actual: %v", expected.ExplicitBounds().AsRaw(), actual.ExplicitBounds().AsRaw()))
	}
	errs = multierr.Append(errs, compareExemplarSlice(expected.Exemplars(), actual.Exemplars()))
	return errs
}

// compareExponentialHistogramDataPointSlice compares each part of two given ExponentialHistogramDataPointSlices and
// returns an error if they don't match. The error describes what didn't match.
func compareExponentialHistogramDataPointSlice(expected, actual pmetric.ExponentialHistogramDataPointSlice) error {
	if expected.Len() != actual.Len() {
		return fmt.Errorf("number of datapoints doesn't match expected: %d, actual: %d", expected.Len(), actual.Len())
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
						fmt.Errorf(`datapoint "%v" expected at index %d, found at index %d`, edp.Attributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected datapoint: %v", edp.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numPoints; i++ {
		if _, ok := matchingDPS[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected datapoint: %v", actual.At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for adp, edp := range matchingDPS {
		errPrefix := fmt.Sprintf(`datapoint "%v"`, adp.Attributes().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareExponentialHistogramDataPoint(edp, adp)))
	}
	return errs
}

// CompareExponentialHistogramDataPoint compares each part of two given ExponentialHistogramDataPoints and returns
// an error if they don't match. The error describes what didn't match.
func CompareExponentialHistogramDataPoint(expected, actual pmetric.ExponentialHistogramDataPoint) error {
	errs := internal.CompareAttributes(expected.Attributes(), actual.Attributes())
	if expected.HasSum() != actual.HasSum() || expected.Sum() != actual.Sum() {
		errStr := "sum doesn't match expected: "
		if expected.HasSum() {
			errStr += fmt.Sprintf("%f", expected.Sum())
		} else {
			errStr += "<unset>"
		}
		errStr += ", actual: "
		if actual.HasSum() {
			errStr += fmt.Sprintf("%f", actual.Sum())
		} else {
			errStr += "<unset>"
		}
		errs = multierr.Append(errs, errors.New(errStr))
	}
	if expected.HasMin() != actual.HasMin() || expected.Min() != actual.Min() {
		errStr := "min doesn't match expected: "
		if expected.HasMin() {
			errStr += fmt.Sprintf("%f", expected.Min())
		} else {
			errStr += "<unset>"
		}
		errStr += ", actual: "
		if actual.HasMin() {
			errStr += fmt.Sprintf("%f", actual.Min())
		} else {
			errStr += "<unset>"
		}
		errs = multierr.Append(errs, errors.New(errStr))
	}
	if expected.HasMax() != actual.HasMax() || expected.Max() != actual.Max() {
		errStr := "max doesn't match expected: "
		if expected.HasMax() {
			errStr += fmt.Sprintf("%f", expected.Min())
		} else {
			errStr += "<unset>"
		}
		errStr += ", actual: "
		if actual.HasMax() {
			errStr += fmt.Sprintf("%f", actual.Min())
		} else {
			errStr += "<unset>"
		}
		errs = multierr.Append(errs, errors.New(errStr))
	}
	if expected.Count() != actual.Count() {
		errs = multierr.Append(errs, fmt.Errorf("count doesn't match expected: %d, actual: %d",
			expected.Count(), actual.Count()))
	}
	if expected.ZeroCount() != actual.ZeroCount() {
		errs = multierr.Append(errs, fmt.Errorf("zero count doesn't match expected: %d, actual: %d",
			expected.ZeroCount(), actual.ZeroCount()))
	}
	if expected.StartTimestamp() != actual.StartTimestamp() {
		errs = multierr.Append(errs, fmt.Errorf("start timestamp doesn't match expected: %d, "+
			"actual: %d", expected.StartTimestamp(), actual.StartTimestamp()))
	}
	if expected.Timestamp() != actual.Timestamp() {
		errs = multierr.Append(errs, fmt.Errorf("timestamp doesn't match expected: %d, actual: %d",
			expected.Timestamp(), actual.Timestamp()))
	}
	if expected.Flags() != actual.Flags() {
		errs = multierr.Append(errs, fmt.Errorf("flags don't match expected: %d, actual: %d",
			expected.Flags(), actual.Flags()))
	}
	if expected.Scale() != actual.Scale() {
		errs = multierr.Append(errs, fmt.Errorf("scale doesn't match expected: %v, actual: %v",
			expected.Scale(), actual.Scale()))
	}
	if expected.Negative().Offset() != actual.Negative().Offset() {
		errs = multierr.Append(errs, fmt.Errorf("negative offset doesn't match expected: %v, "+
			"actual: %v", expected.Negative().Offset(), actual.Negative().Offset()))
	}
	if !reflect.DeepEqual(expected.Negative().BucketCounts(), actual.Negative().BucketCounts()) {
		errs = multierr.Append(errs, fmt.Errorf("negative bucket counts don't match expected: %v, "+
			"actual: %v", expected.Negative().BucketCounts().AsRaw(), actual.Negative().BucketCounts().AsRaw()))
	}
	if expected.Positive().Offset() != actual.Positive().Offset() {
		errs = multierr.Append(errs, fmt.Errorf("positive offset doesn't match expected: %v, "+
			"actual: %v", expected.Positive().Offset(), actual.Positive().Offset()))
	}
	if !reflect.DeepEqual(expected.Positive().BucketCounts(), actual.Positive().BucketCounts()) {
		errs = multierr.Append(errs, fmt.Errorf("positive bucket counts don't match expected: %v, "+
			"actual: %v", expected.Positive().BucketCounts().AsRaw(), actual.Positive().BucketCounts().AsRaw()))
	}
	errs = multierr.Append(errs, compareExemplarSlice(expected.Exemplars(), actual.Exemplars()))
	return errs
}

// compareSummaryDataPointSlices compares each part of two given SummaryDataPoint slices and returns
// an error if they don't match. The error describes what didn't match.
func compareSummaryDataPointSlices(expected, actual pmetric.SummaryDataPointSlice) error {
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
						fmt.Errorf(`datapoint "%v" expected at index %d, found at index %d`, edp.Attributes().AsRaw(), e, a))
				}
				break
			}
		}

		if !foundMatch {
			errs = multierr.Append(errs, fmt.Errorf("missing expected datapoint: %v", edp.Attributes().AsRaw()))
		}
	}

	for i := 0; i < numPoints; i++ {
		if _, ok := matchingDPS[actual.At(i)]; !ok {
			errs = multierr.Append(errs, fmt.Errorf("unexpected datapoint: %v", actual.At(i).Attributes().AsRaw()))
		}
	}

	if errs != nil {
		return errs
	}
	if outOfOrderErrs != nil {
		return outOfOrderErrs
	}

	for adp, edp := range matchingDPS {
		errPrefix := fmt.Sprintf(`datapoint "%v"`, adp.Attributes().AsRaw())
		errs = multierr.Append(errs, internal.AddErrPrefix(errPrefix, CompareSummaryDataPoint(edp, adp)))
	}
	return errs
}

// CompareSummaryDataPoint compares each part of two given SummaryDataPoint and returns
// an error if they don't match. The error describes what didn't match.
func CompareSummaryDataPoint(expected, actual pmetric.SummaryDataPoint) error {
	errs := internal.CompareAttributes(expected.Attributes(), actual.Attributes())

	if expected.Count() != actual.Count() {
		errs = multierr.Append(errs, fmt.Errorf("count doesn't match expected: %d, actual: %d",
			expected.Count(), actual.Count()))
	}
	if expected.Sum() != actual.Sum() {
		errs = multierr.Append(errs, fmt.Errorf("sum doesn't match expected: %f, actual: %f",
			expected.Sum(), actual.Sum()))
	}
	if expected.StartTimestamp() != actual.StartTimestamp() {
		errs = multierr.Append(errs, fmt.Errorf("start timestamp doesn't match expected: %d, "+
			"actual: %d", expected.StartTimestamp(), actual.StartTimestamp()))
	}
	if expected.Timestamp() != actual.Timestamp() {
		errs = multierr.Append(errs, fmt.Errorf("timestamp doesn't match expected: %d, actual: %d",
			expected.Timestamp(), actual.Timestamp()))
	}
	if expected.Flags() != actual.Flags() {
		errs = multierr.Append(errs, fmt.Errorf("flags don't match expected: %d, actual: %d",
			expected.Flags(), actual.Flags()))
	}
	if expected.QuantileValues().Len() != actual.QuantileValues().Len() {
		errs = multierr.Append(errs, fmt.Errorf("quantile values length doesn't match expected: %d, "+
			"actual: %d", expected.QuantileValues().Len(), actual.QuantileValues().Len()))
		return errs
	}

	for i := 0; i < expected.QuantileValues().Len(); i++ {
		eqv, acv := expected.QuantileValues().At(i), actual.QuantileValues().At(i)
		if eqv.Quantile() != acv.Quantile() {
			errs = multierr.Append(errs, fmt.Errorf("quantile doesn't match expected: %f, "+
				"actual: %f", eqv.Quantile(), acv.Quantile()))
		}
		if eqv.Value() != acv.Value() {
			errs = multierr.Append(errs, fmt.Errorf("value at quantile %f doesn't match expected: %f, actual: %f",
				eqv.Quantile(), eqv.Value(), acv.Value()))
		}
	}

	return errs
}
