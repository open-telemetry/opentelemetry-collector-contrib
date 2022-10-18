// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
)

type accumulatedValue struct {
	// value contains a metric with exactly one aggregated datapoint.
	value pmetric.Metric

	// resourceAttrs contain the resource attributes. They are used to output instance and job labels.
	resourceAttrs pcommon.Map

	// updated indicates when metric was last changed.
	updated time.Time

	scope pcommon.InstrumentationScope
}

// accumulator stores aggragated values of incoming metrics
type accumulator interface {
	// Accumulate stores aggragated metric values
	Accumulate(resourceMetrics pmetric.ResourceMetrics) (processed int)
	// Collect returns a slice with relevant aggregated metrics and their resource attributes.
	// The number or metrics and attributes returned will be the same.
	Collect() (metrics []pmetric.Metric, resourceAttrs []pcommon.Map)
}

// LastValueAccumulator keeps last value for accumulated metrics
type lastValueAccumulator struct {
	logger *zap.Logger

	registeredMetrics sync.Map

	// metricExpiration contains duration for which metric
	// should be served after it was updated
	metricExpiration time.Duration
}

// NewAccumulator returns LastValueAccumulator
func newAccumulator(logger *zap.Logger, metricExpiration time.Duration) accumulator {
	return &lastValueAccumulator{
		logger:           logger,
		metricExpiration: metricExpiration,
	}
}

// Accumulate stores one datapoint per metric
func (a *lastValueAccumulator) Accumulate(rm pmetric.ResourceMetrics) (n int) {
	now := time.Now()
	ilms := rm.ScopeMetrics()
	resourceAttrs := rm.Resource().Attributes()

	for i := 0; i < ilms.Len(); i++ {
		ilm := ilms.At(i)

		metrics := ilm.Metrics()
		for j := 0; j < metrics.Len(); j++ {
			n += a.addMetric(metrics.At(j), ilm.Scope(), resourceAttrs, now)
		}
	}

	return
}

func (a *lastValueAccumulator) addMetric(metric pmetric.Metric, il pcommon.InstrumentationScope, resourceAttrs pcommon.Map, now time.Time) int {
	a.logger.Debug(fmt.Sprintf("accumulating metric: %s", metric.Name()))

	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		return a.accumulateGauge(metric, il, resourceAttrs, now)
	case pmetric.MetricDataTypeSum:
		return a.accumulateSum(metric, il, resourceAttrs, now)
	case pmetric.MetricDataTypeHistogram:
		return a.accumulateHistogram(metric, il, resourceAttrs, now)
	case pmetric.MetricDataTypeSummary:
		return a.accumulateSummary(metric, il, resourceAttrs, now)
	default:
		a.logger.With(
			zap.String("data_type", string(metric.DataType())),
			zap.String("metric_name", metric.Name()),
		).Error("failed to translate metric")
	}

	return 0
}

func (a *lastValueAccumulator) accumulateSummary(metric pmetric.Metric, il pcommon.InstrumentationScope, resourceAttrs pcommon.Map, now time.Time) (n int) {
	dps := metric.Summary().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		ip := dps.At(i)

		signature := timeseriesSignature(il.Name(), metric, ip.Attributes(), resourceAttrs)
		if ip.Flags().NoRecordedValue() {
			a.registeredMetrics.Delete(signature)
			return 0
		}

		v, ok := a.registeredMetrics.Load(signature)
		stalePoint := ok &&
			ip.Timestamp().AsTime().Before(v.(*accumulatedValue).value.Summary().DataPoints().At(0).Timestamp().AsTime())

		if stalePoint {
			// Only keep this datapoint if it has a later timestamp.
			continue
		}

		mm := createMetric(metric)
		ip.CopyTo(mm.Summary().DataPoints().AppendEmpty())
		a.registeredMetrics.Store(signature, &accumulatedValue{value: mm, resourceAttrs: resourceAttrs, scope: il, updated: now})
		n++
	}

	return n
}

func (a *lastValueAccumulator) accumulateGauge(metric pmetric.Metric, il pcommon.InstrumentationScope, resourceAttrs pcommon.Map, now time.Time) (n int) {
	dps := metric.Gauge().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		ip := dps.At(i)

		signature := timeseriesSignature(il.Name(), metric, ip.Attributes(), resourceAttrs)
		if ip.Flags().NoRecordedValue() {
			a.registeredMetrics.Delete(signature)
			return 0
		}

		v, ok := a.registeredMetrics.Load(signature)
		if !ok {
			m := createMetric(metric)
			ip.CopyTo(m.Gauge().DataPoints().AppendEmpty())
			a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
			n++
			continue
		}
		mv := v.(*accumulatedValue)

		if ip.Timestamp().AsTime().Before(mv.value.Gauge().DataPoints().At(0).Timestamp().AsTime()) {
			// only keep datapoint with latest timestamp
			continue
		}

		m := createMetric(metric)
		ip.CopyTo(m.Gauge().DataPoints().AppendEmpty())
		a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
		n++
	}
	return
}

func (a *lastValueAccumulator) accumulateSum(metric pmetric.Metric, il pcommon.InstrumentationScope, resourceAttrs pcommon.Map, now time.Time) (n int) {
	sum := metric.Sum()

	// Drop metrics with unspecified aggregations
	if sum.AggregationTemporality() == pmetric.MetricAggregationTemporalityUnspecified {
		return
	}

	// Drop non-monotonic and non-cumulative metrics
	if sum.AggregationTemporality() == pmetric.MetricAggregationTemporalityDelta && !sum.IsMonotonic() {
		return
	}

	dps := sum.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		ip := dps.At(i)

		signature := timeseriesSignature(il.Name(), metric, ip.Attributes(), resourceAttrs)
		if ip.Flags().NoRecordedValue() {
			a.registeredMetrics.Delete(signature)
			return 0
		}

		v, ok := a.registeredMetrics.Load(signature)
		if !ok {
			m := createMetric(metric)
			m.Sum().SetIsMonotonic(metric.Sum().IsMonotonic())
			m.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
			ip.CopyTo(m.Sum().DataPoints().AppendEmpty())
			a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
			n++
			continue
		}
		mv := v.(*accumulatedValue)

		m := createMetric(metric)
		m.Sum().SetIsMonotonic(metric.Sum().IsMonotonic())
		m.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)

		switch sum.AggregationTemporality() {
		case pmetric.MetricAggregationTemporalityDelta:
			AccumulateNumericValues(mv.value.Sum().DataPoints().At(0), ip, m.Sum().DataPoints().AppendEmpty())
		case pmetric.MetricAggregationTemporalityCumulative:
			if ip.Timestamp().AsTime().Before(mv.value.Sum().DataPoints().At(0).Timestamp().AsTime()) {
				// only keep datapoint with latest timestamp
				continue
			}

			ip.CopyTo(m.Sum().DataPoints().AppendEmpty())
		default:
			// unsupported temporality
			continue
		}
		a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
		n++
	}

	return
}

func (a *lastValueAccumulator) accumulateHistogram(metric pmetric.Metric, il pcommon.InstrumentationScope, resourceAttrs pcommon.Map, now time.Time) (n int) {
	histogram := metric.Histogram()

	dps := histogram.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		ip := dps.At(i)

		signature := timeseriesSignature(il.Name(), metric, ip.Attributes(), resourceAttrs)
		if ip.Flags().NoRecordedValue() {
			a.registeredMetrics.Delete(signature)
			return 0
		}

		v, ok := a.registeredMetrics.Load(signature)
		if !ok {
			m := createMetric(metric)
			ip.CopyTo(m.Histogram().DataPoints().AppendEmpty())
			m.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
			a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
			n++
			continue
		}
		mv := v.(*accumulatedValue)

		m := createMetric(metric)
		m.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)

		switch histogram.AggregationTemporality() {
		case pmetric.MetricAggregationTemporalityDelta:
			AccumulateHistogramValues(mv.value.Histogram().DataPoints().At(0), ip, m.Histogram().DataPoints().AppendEmpty())
		case pmetric.MetricAggregationTemporalityCumulative:
			if ip.Timestamp().AsTime().Before(mv.value.Histogram().DataPoints().At(0).Timestamp().AsTime()) {
				// only keep datapoint with latest timestamp
				continue
			}

			ip.CopyTo(m.Histogram().DataPoints().AppendEmpty())
		default:
			// unsupported temporality
			continue
		}
		a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
		n++
	}
	return
}

// Collect returns a slice with relevant aggregated metrics and their resource attributes.
func (a *lastValueAccumulator) Collect() ([]pmetric.Metric, []pcommon.Map) {
	a.logger.Debug("Accumulator collect called")

	var metrics []pmetric.Metric
	var resourceAttrs []pcommon.Map
	expirationTime := time.Now().Add(-a.metricExpiration)

	a.registeredMetrics.Range(func(key, value interface{}) bool {
		v := value.(*accumulatedValue)
		if expirationTime.After(v.updated) {
			a.logger.Debug(fmt.Sprintf("metric expired: %s", v.value.Name()))
			a.registeredMetrics.Delete(key)
			return true
		}

		metrics = append(metrics, v.value)
		resourceAttrs = append(resourceAttrs, v.resourceAttrs)
		return true
	})

	return metrics, resourceAttrs
}

func timeseriesSignature(ilmName string, metric pmetric.Metric, attributes pcommon.Map, resourceAttrs pcommon.Map) string {
	var b strings.Builder
	b.WriteString(metric.DataType().String())
	b.WriteString("*" + ilmName)
	b.WriteString("*" + metric.Name())
	attributes.Sort().Range(func(k string, v pcommon.Value) bool {
		b.WriteString("*" + k + "*" + v.AsString())
		return true
	})

	// We only include the job and instance labels in the final output. So we should only construct the signature based on those.
	if serviceName, ok := resourceAttrs.Get(conventions.AttributeServiceName); ok {
		val := serviceName.AsString()
		if serviceNamespace, ok := resourceAttrs.Get(conventions.AttributeServiceNamespace); ok {
			val = fmt.Sprintf("%s/%s", serviceNamespace.AsString(), val)
		}
		b.WriteString("*" + model.JobLabel + "*" + val)
	}
	if instance, ok := resourceAttrs.Get(conventions.AttributeServiceInstanceID); ok {
		b.WriteString("*" + model.InstanceLabel + "*" + instance.AsString())
	}

	return b.String()
}

func createMetric(metric pmetric.Metric) pmetric.Metric {
	m := pmetric.NewMetric()
	m.SetName(metric.Name())
	m.SetDescription(metric.Description())
	m.SetUnit(metric.Unit())
	m.SetDataType(metric.DataType())

	return m
}

func AccumulateNumericValues(prev, current, dest pmetric.NumberDataPoint) {
	if current.StartTimestamp().AsTime().Before(prev.StartTimestamp().AsTime()) {
		dest.SetStartTimestamp(current.StartTimestamp())
	} else {
		dest.SetStartTimestamp(prev.StartTimestamp())
	}

	older := prev
	newer := current
	if current.Timestamp().AsTime().Before(prev.Timestamp().AsTime()) {
		older = current
		newer = prev
	}

	newer.Attributes().CopyTo(dest.Attributes())
	dest.SetTimestamp(newer.Timestamp())

	switch newer.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		dest.SetDoubleVal(newer.DoubleVal() + older.DoubleVal())
	case pmetric.NumberDataPointValueTypeInt:
		dest.SetIntVal(newer.IntVal() + older.IntVal())
	}
}

func AccumulateHistogramValues(prev, current, dest pmetric.HistogramDataPoint) {
	if current.StartTimestamp().AsTime().Before(prev.StartTimestamp().AsTime()) {
		dest.SetStartTimestamp(current.StartTimestamp())
	} else {
		dest.SetStartTimestamp(prev.StartTimestamp())
	}

	older := prev
	newer := current
	if current.Timestamp().AsTime().Before(prev.Timestamp().AsTime()) {
		older = current
		newer = prev
	}

	newer.Attributes().CopyTo(dest.Attributes())
	dest.SetTimestamp(newer.Timestamp())

	match := true
	if older.ExplicitBounds().Len() == newer.ExplicitBounds().Len() {
		for i := 0; i < newer.BucketCounts().Len(); i++ {
			if older.ExplicitBounds().At(i) != newer.ExplicitBounds().At(i) {
				match = false
				break
			}
		}
	} else {
		match = false
	}

	if match {
		dest.SetCount(newer.Count() + older.Count())
		if newer.HasSum() {
			dest.SetSum(newer.Sum() + older.Sum())
		}

		counts := make([]uint64, newer.BucketCounts().Len())
		for i := 0; i < newer.BucketCounts().Len(); i++ {
			counts[i] = newer.BucketCounts().At(i) + older.BucketCounts().At(i)
		}
		dest.SetBucketCounts(pcommon.NewImmutableUInt64Slice(counts))
	} else {
		// use new value if bucket bounds do not match
		dest.SetCount(newer.Count())
		if newer.HasSum() {
			dest.SetSum(newer.Sum())
		}

		dest.SetBucketCounts(newer.BucketCounts())
	}

	dest.SetExplicitBounds(newer.ExplicitBounds())
}
