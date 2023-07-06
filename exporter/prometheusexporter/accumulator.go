// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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

	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		return a.accumulateGauge(metric, il, resourceAttrs, now)
	case pmetric.MetricTypeSum:
		return a.accumulateSum(metric, il, resourceAttrs, now)
	case pmetric.MetricTypeHistogram:
		return a.accumulateHistogram(metric, il, resourceAttrs, now)
	case pmetric.MetricTypeSummary:
		return a.accumulateSummary(metric, il, resourceAttrs, now)
	default:
		a.logger.With(
			zap.String("data_type", string(metric.Type())),
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

		m := copyMetricMetadata(metric)
		ip.CopyTo(m.SetEmptySummary().DataPoints().AppendEmpty())
		a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
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
			m := copyMetricMetadata(metric)
			ip.CopyTo(m.SetEmptyGauge().DataPoints().AppendEmpty())
			a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
			n++
			continue
		}
		mv := v.(*accumulatedValue)

		if ip.Timestamp().AsTime().Before(mv.value.Gauge().DataPoints().At(0).Timestamp().AsTime()) {
			// only keep datapoint with latest timestamp
			continue
		}

		m := copyMetricMetadata(metric)
		ip.CopyTo(m.SetEmptyGauge().DataPoints().AppendEmpty())
		a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
		n++
	}
	return
}

func (a *lastValueAccumulator) accumulateSum(metric pmetric.Metric, il pcommon.InstrumentationScope, resourceAttrs pcommon.Map, now time.Time) (n int) {
	sum := metric.Sum()

	// Drop metrics with unspecified aggregations
	if sum.AggregationTemporality() == pmetric.AggregationTemporalityUnspecified {
		return
	}

	// Drop non-monotonic and non-cumulative metrics
	if sum.AggregationTemporality() == pmetric.AggregationTemporalityDelta && !sum.IsMonotonic() {
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
			m := copyMetricMetadata(metric)
			m.SetEmptySum().SetIsMonotonic(metric.Sum().IsMonotonic())
			m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			ip.CopyTo(m.Sum().DataPoints().AppendEmpty())
			a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
			n++
			continue
		}
		mv := v.(*accumulatedValue)

		if ip.Timestamp().AsTime().Before(mv.value.Sum().DataPoints().At(0).Timestamp().AsTime()) {
			// only keep datapoint with latest timestamp
			continue
		}

		// Delta-to-Cumulative
		if sum.AggregationTemporality() == pmetric.AggregationTemporalityDelta && ip.StartTimestamp() == mv.value.Sum().DataPoints().At(0).Timestamp() {
			ip.SetStartTimestamp(mv.value.Sum().DataPoints().At(0).StartTimestamp())
			switch ip.ValueType() {
			case pmetric.NumberDataPointValueTypeInt:
				ip.SetIntValue(ip.IntValue() + mv.value.Sum().DataPoints().At(0).IntValue())
			case pmetric.NumberDataPointValueTypeDouble:
				ip.SetDoubleValue(ip.DoubleValue() + mv.value.Sum().DataPoints().At(0).DoubleValue())
			}
		}

		m := copyMetricMetadata(metric)
		m.SetEmptySum().SetIsMonotonic(metric.Sum().IsMonotonic())
		m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		ip.CopyTo(m.Sum().DataPoints().AppendEmpty())
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
			// first data point
			m := copyMetricMetadata(metric)
			ip.CopyTo(m.SetEmptyHistogram().DataPoints().AppendEmpty())
			m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scope: il, updated: now})
			n++
			continue
		}
		mv := v.(*accumulatedValue)

		m := copyMetricMetadata(metric)
		m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		switch histogram.AggregationTemporality() {
		case pmetric.AggregationTemporalityDelta:
			pp := mv.value.Histogram().DataPoints().At(0) // previous aggregated value for time range
			if ip.StartTimestamp().AsTime() != pp.StartTimestamp().AsTime() {
				// treat misalgnment as restart and reset, or violation of single-writer principle and drop
				if ip.StartTimestamp().AsTime().After(pp.Timestamp().AsTime()) {
					ip.CopyTo(m.Histogram().DataPoints().AppendEmpty())
				} else {
					a.logger.With(
						zap.String("metric_name", metric.Name()),
					).Warn("Dropped misaligned histogram datapoint")
					continue
				}
			} else if ip.HasSum() != pp.HasSum() {
				a.logger.With(
					zap.String("metric_name", metric.Name()),
				).Warn("Dropped histogram data point due to disagreement on sum tracking")
				continue
			} else {
				accumulateHistogramValues(pp, ip, m.Histogram().DataPoints().AppendEmpty())
			}
		case pmetric.AggregationTemporalityCumulative:
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
	b.WriteString(metric.Type().String())
	b.WriteString("*" + ilmName)
	b.WriteString("*" + metric.Name())
	attrs := make([]string, 0, attributes.Len())
	attributes.Range(func(k string, v pcommon.Value) bool {
		attrs = append(attrs, k+"*"+v.AsString())
		return true
	})
	sort.Strings(attrs)
	b.WriteString("*" + strings.Join(attrs, "*"))
	if job, ok := extractJob(resourceAttrs); ok {
		b.WriteString("*" + model.JobLabel + "*" + job)
	}
	if instance, ok := extractInstance(resourceAttrs); ok {
		b.WriteString("*" + model.InstanceLabel + "*" + instance)
	}
	return b.String()
}

func copyMetricMetadata(metric pmetric.Metric) pmetric.Metric {
	m := pmetric.NewMetric()
	m.SetName(metric.Name())
	m.SetDescription(metric.Description())
	m.SetUnit(metric.Unit())

	return m
}

func accumulateHistogramValues(prev, current, dest pmetric.HistogramDataPoint) {
	dest.SetStartTimestamp(prev.StartTimestamp())

	older := prev
	newer := current
	if current.Timestamp().AsTime().Before(prev.Timestamp().AsTime()) {
		older = current
		newer = prev
	}

	newer.Attributes().CopyTo(dest.Attributes())
	dest.SetTimestamp(newer.Timestamp())

	// checking for bucket boundary alignment, optionally re-aggregate on newer boundaries
	match := older.ExplicitBounds().Len() == newer.ExplicitBounds().Len()
	for i := 0; match && i < newer.ExplicitBounds().Len(); i++ {
		match = older.ExplicitBounds().At(i) == newer.ExplicitBounds().At(i)
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
		dest.BucketCounts().FromRaw(counts)
	} else {
		// use new value if bucket bounds do not match
		dest.SetCount(newer.Count())
		if newer.HasSum() {
			dest.SetSum(newer.Sum())
		}

		dest.BucketCounts().FromRaw(newer.BucketCounts().AsRaw())
	}

	dest.ExplicitBounds().FromRaw(newer.ExplicitBounds().AsRaw())
}
