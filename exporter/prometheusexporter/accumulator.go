// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// stringBuilderPool is a pool of strings.Builder instances to reduce allocations
var stringBuilderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

type attributeBuffer struct {
	attrs []string
}

var attributeBufferPool = sync.Pool{
	New: func() any {
		return &attributeBuffer{
			// Pre-allocate capacity of 32 to avoid reallocations for typical metrics
			// which usually have 5-20 attributes. For metrics with >32 attributes,
			// the slice will grow automatically and retain the larger capacity for reuse.
			attrs: make([]string, 0, 32),
		}
	},
}

type accumulatedValue struct {
	// value contains a metric with exactly one aggregated datapoint.
	value pmetric.Metric

	// resourceAttrs contain the resource attributes. They are used to output instance and job labels.
	resourceAttrs pcommon.Map

	// updated indicates when metric was last changed.
	updated time.Time

	scopeName       string
	scopeVersion    string
	scopeSchemaURL  string
	scopeAttributes pcommon.Map
}

// accumulator stores aggregated values of incoming metrics
type accumulator interface {
	// Accumulate stores aggregated metric values
	Accumulate(resourceMetrics pmetric.ResourceMetrics) (processed int)
	// Collect returns a slice with relevant aggregated metrics and their resource attributes.
	// The number or metrics and attributes returned will be the same.
	Collect() (metrics []pmetric.Metric, resourceAttrs []pcommon.Map, scopeNames, scopeVersions, scopeSchemaURLs []string, scopeAttributes []pcommon.Map)
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
			n += a.addMetric(metrics.At(j), ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), resourceAttrs, now)
		}
	}

	return n
}

func (a *lastValueAccumulator) addMetric(metric pmetric.Metric, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes, resourceAttrs pcommon.Map, now time.Time) int {
	a.logger.Debug(fmt.Sprintf("accumulating metric: %s", metric.Name()))

	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		return a.accumulateGauge(metric, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, resourceAttrs, now)
	case pmetric.MetricTypeSum:
		return a.accumulateSum(metric, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, resourceAttrs, now)
	case pmetric.MetricTypeHistogram:
		return a.accumulateHistogram(metric, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, resourceAttrs, now)
	case pmetric.MetricTypeSummary:
		return a.accumulateSummary(metric, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, resourceAttrs, now)
	case pmetric.MetricTypeExponentialHistogram:
		return a.accumulateExponentialHistogram(metric, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, resourceAttrs, now)
	default:
		a.logger.With(
			zap.String("data_type", metric.Type().String()),
			zap.String("metric_name", metric.Name()),
		).Error("failed to translate metric")
	}

	return 0
}

func (a *lastValueAccumulator) accumulateSummary(metric pmetric.Metric, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes, resourceAttrs pcommon.Map, now time.Time) (n int) {
	dps := metric.Summary().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		ip := dps.At(i)

		signature := timeseriesSignature(scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, metric, ip.Attributes(), resourceAttrs)
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
		a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scopeName: scopeName, scopeVersion: scopeVersion, scopeSchemaURL: scopeSchemaURL, scopeAttributes: scopeAttributes, updated: now})
		n++
	}

	return n
}

func (a *lastValueAccumulator) accumulateGauge(metric pmetric.Metric, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes, resourceAttrs pcommon.Map, now time.Time) (n int) {
	dps := metric.Gauge().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		ip := dps.At(i)

		signature := timeseriesSignature(scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, metric, ip.Attributes(), resourceAttrs)
		if ip.Flags().NoRecordedValue() {
			a.registeredMetrics.Delete(signature)
			return 0
		}

		v, ok := a.registeredMetrics.Load(signature)
		if !ok {
			m := copyMetricMetadata(metric)
			ip.CopyTo(m.SetEmptyGauge().DataPoints().AppendEmpty())
			a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scopeName: scopeName, scopeVersion: scopeVersion, scopeSchemaURL: scopeSchemaURL, scopeAttributes: scopeAttributes, updated: now})
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
		a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scopeName: scopeName, scopeVersion: scopeVersion, scopeSchemaURL: scopeSchemaURL, scopeAttributes: scopeAttributes, updated: now})
		n++
	}
	return n
}

func (a *lastValueAccumulator) accumulateSum(metric pmetric.Metric, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes, resourceAttrs pcommon.Map, now time.Time) (n int) {
	doubleSum := metric.Sum()

	// Drop metrics with unspecified aggregations
	if doubleSum.AggregationTemporality() == pmetric.AggregationTemporalityUnspecified {
		return n
	}

	// Drop non-monotonic and non-cumulative metrics
	if doubleSum.AggregationTemporality() == pmetric.AggregationTemporalityDelta && !doubleSum.IsMonotonic() {
		a.logger.Debug("refusing non-monotonic delta sum metric",
			zap.String("metric_name", metric.Name()),
			zap.Int("data_points_refused", doubleSum.DataPoints().Len()),
			zap.String("reason", "non-monotonic sum with delta aggregation temporality is not supported"))
		return 0
	}

	dps := doubleSum.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		ip := dps.At(i)

		signature := timeseriesSignature(scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, metric, ip.Attributes(), resourceAttrs)
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
			a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scopeName: scopeName, scopeVersion: scopeVersion, scopeSchemaURL: scopeSchemaURL, scopeAttributes: scopeAttributes, updated: now})
			n++
			continue
		}
		mv := v.(*accumulatedValue)

		if ip.Timestamp().AsTime().Before(mv.value.Sum().DataPoints().At(0).Timestamp().AsTime()) {
			// only keep datapoint with latest timestamp
			continue
		}

		// Delta-to-Cumulative
		if doubleSum.AggregationTemporality() == pmetric.AggregationTemporalityDelta && ip.StartTimestamp() == mv.value.Sum().DataPoints().At(0).Timestamp() {
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
		a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scopeName: scopeName, scopeVersion: scopeVersion, scopeSchemaURL: scopeSchemaURL, scopeAttributes: scopeAttributes, updated: now})
		n++
	}
	return n
}

func (a *lastValueAccumulator) accumulateHistogram(metric pmetric.Metric, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes, resourceAttrs pcommon.Map, now time.Time) (n int) {
	histogram := metric.Histogram()
	a.logger.Debug("Accumulate histogram.....")
	dps := histogram.DataPoints()

	for i := 0; i < dps.Len(); i++ {
		ip := dps.At(i)

		signature := timeseriesSignature(scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, metric, ip.Attributes(), resourceAttrs) // uniquely identify this time series you are accumulating for
		if ip.Flags().NoRecordedValue() {
			a.registeredMetrics.Delete(signature)
			return 0
		}

		v, ok := a.registeredMetrics.Load(signature) // a accumulates metric values for all times series. Get value for particular time series
		if !ok {
			// first data point
			m := copyMetricMetadata(metric)
			ip.CopyTo(m.SetEmptyHistogram().DataPoints().AppendEmpty())
			m.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scopeName: scopeName, scopeVersion: scopeVersion, scopeSchemaURL: scopeSchemaURL, scopeAttributes: scopeAttributes, updated: now})
			n++
			continue
		}
		mv := v.(*accumulatedValue)

		m := copyMetricMetadata(metric)
		m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		switch histogram.AggregationTemporality() {
		case pmetric.AggregationTemporalityDelta:
			pp := mv.value.Histogram().DataPoints().At(0) // previous aggregated value for time range
			if ip.StartTimestamp().AsTime() != pp.Timestamp().AsTime() {
				// treat misalignment as restart and reset, or violation of single-writer principle and drop
				a.logger.With(
					zap.String("ip_start_time", ip.StartTimestamp().String()),
					zap.String("pp_start_time", pp.StartTimestamp().String()),
					zap.String("pp_timestamp", pp.Timestamp().String()),
					zap.String("ip_timestamp", ip.Timestamp().String()),
				).Warn("Misaligned starting timestamps")
				if !ip.StartTimestamp().AsTime().After(pp.Timestamp().AsTime()) {
					a.logger.With(
						zap.String("metric_name", metric.Name()),
					).Warn("Dropped misaligned histogram datapoint")
					continue
				}
				a.logger.Debug("treating it like reset")
				ip.CopyTo(m.Histogram().DataPoints().AppendEmpty())
			} else {
				a.logger.Debug("Accumulate another histogram datapoint")
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
		a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scopeName: scopeName, scopeVersion: scopeVersion, scopeSchemaURL: scopeSchemaURL, scopeAttributes: scopeAttributes, updated: now})
		n++
	}
	return n
}

func (a *lastValueAccumulator) accumulateExponentialHistogram(metric pmetric.Metric, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes, resourceAttrs pcommon.Map, now time.Time) (n int) {
	expHistogram := metric.ExponentialHistogram()
	a.logger.Debug("Accumulate native histogram.....")
	dps := expHistogram.DataPoints()

	for i := 0; i < dps.Len(); i++ {
		ip := dps.At(i)
		signature := timeseriesSignature(scopeName, scopeVersion, scopeSchemaURL, scopeAttributes, metric, ip.Attributes(), resourceAttrs) // uniquely identify this time series you are accumulating for
		if ip.Flags().NoRecordedValue() {
			a.registeredMetrics.Delete(signature)
			return 0
		}

		v, ok := a.registeredMetrics.Load(signature) // a accumulates metric values for all times series. Get value for particular time series
		if !ok {
			// first data point
			m := copyMetricMetadata(metric)
			ip.CopyTo(m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty())
			m.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scopeName: scopeName, scopeVersion: scopeVersion, scopeSchemaURL: scopeSchemaURL, scopeAttributes: scopeAttributes, updated: now})
			n++
			continue
		}
		mv := v.(*accumulatedValue)

		m := copyMetricMetadata(metric)
		m.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		switch expHistogram.AggregationTemporality() {
		case pmetric.AggregationTemporalityDelta:
			pp := mv.value.ExponentialHistogram().DataPoints().At(0) // previous aggregated value for time range
			if ip.StartTimestamp().AsTime() != pp.Timestamp().AsTime() {
				// treat misalignment as restart and reset, or violation of single-writer principle and drop
				a.logger.With(
					zap.String("ip_start_time", ip.StartTimestamp().String()),
					zap.String("pp_start_time", pp.StartTimestamp().String()),
					zap.String("pp_timestamp", pp.Timestamp().String()),
					zap.String("ip_timestamp", ip.Timestamp().String()),
				).Warn("Misaligned starting timestamps")
				if !ip.StartTimestamp().AsTime().After(pp.Timestamp().AsTime()) {
					a.logger.With(
						zap.String("metric_name", metric.Name()),
					).Warn("Dropped misaligned histogram datapoint")
					continue
				}
				a.logger.Debug("Treating it like reset")
				ip.CopyTo(m.ExponentialHistogram().DataPoints().AppendEmpty())
			} else {
				a.logger.Debug("Accumulate another histogram datapoint")
				accumulateExponentialHistogramValues(pp, ip, m.ExponentialHistogram().DataPoints().AppendEmpty())
			}
		case pmetric.AggregationTemporalityCumulative:
			if ip.Timestamp().AsTime().Before(mv.value.ExponentialHistogram().DataPoints().At(0).Timestamp().AsTime()) {
				// only keep datapoint with latest timestamp
				continue
			}

			ip.CopyTo(m.ExponentialHistogram().DataPoints().AppendEmpty())
		default:
			// unsupported temporality
			continue
		}

		// Store the updated metric and advance count
		a.registeredMetrics.Store(signature, &accumulatedValue{value: m, resourceAttrs: resourceAttrs, scopeName: scopeName, scopeVersion: scopeVersion, scopeSchemaURL: scopeSchemaURL, scopeAttributes: scopeAttributes, updated: now})
		n++
	}

	return n
}

// Collect returns a slice with relevant aggregated metrics and their resource attributes.
func (a *lastValueAccumulator) Collect() ([]pmetric.Metric, []pcommon.Map, []string, []string, []string, []pcommon.Map) {
	a.logger.Debug("Accumulator collect called")

	var metrics []pmetric.Metric
	var resourceAttrs []pcommon.Map
	var scopeNames []string
	var scopeVersions []string
	var scopeSchemaURLs []string
	var scopeAttributes []pcommon.Map
	expirationTime := time.Now().Add(-a.metricExpiration)

	a.registeredMetrics.Range(func(key, value any) bool {
		v := value.(*accumulatedValue)
		if expirationTime.After(v.updated) {
			a.logger.Debug(fmt.Sprintf("metric expired: %s", v.value.Name()))
			a.registeredMetrics.Delete(key)
			return true
		}

		metrics = append(metrics, v.value)
		resourceAttrs = append(resourceAttrs, v.resourceAttrs)
		scopeNames = append(scopeNames, v.scopeName)
		scopeVersions = append(scopeVersions, v.scopeVersion)
		scopeSchemaURLs = append(scopeSchemaURLs, v.scopeSchemaURL)
		scopeAttributes = append(scopeAttributes, v.scopeAttributes)
		return true
	})

	return metrics, resourceAttrs, scopeNames, scopeVersions, scopeSchemaURLs, scopeAttributes
}

func timeseriesSignature(scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes pcommon.Map, metric pmetric.Metric, attributes, resourceAttrs pcommon.Map) string {
	// Get a string builder from the pool
	sb := stringBuilderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringBuilderPool.Put(sb)
	}()

	// Get an attribute buffer from the pool for sorting
	attrBuf := attributeBufferPool.Get().(*attributeBuffer)
	defer func() {
		attrBuf.attrs = attrBuf.attrs[:0]
		attributeBufferPool.Put(attrBuf)
	}()

	// Build signature from metric name and type
	sb.WriteString(metric.Name())
	sb.WriteString("*")
	sb.WriteString(metric.Type().String())
	sb.WriteString("*")
	sb.WriteString(scopeName)
	sb.WriteString("*")
	sb.WriteString(scopeVersion)
	sb.WriteString("*")
	sb.WriteString(scopeSchemaURL)
	sb.WriteString("*")

	// Add scope attributes in sorted order for consistency
	if scopeAttributes.Len() > 0 {
		attrBuf.attrs = attrBuf.attrs[:0]
		for k, v := range scopeAttributes.All() {
			attrBuf.attrs = append(attrBuf.attrs, k+"*"+v.AsString())
		}
		sort.Strings(attrBuf.attrs)
		for _, attr := range attrBuf.attrs {
			sb.WriteString(attr)
			sb.WriteString("*")
		}
	}

	// Add metric attributes in sorted order
	if attributes.Len() > 0 {
		attrBuf.attrs = attrBuf.attrs[:0]
		for k, v := range attributes.All() {
			attrBuf.attrs = append(attrBuf.attrs, k+"*"+v.AsString())
		}
		sort.Strings(attrBuf.attrs)
		for _, attr := range attrBuf.attrs {
			sb.WriteString(attr)
			sb.WriteString("*")
		}
	}

	// Add job and instance labels
	if job, ok := extractJob(resourceAttrs); ok {
		sb.WriteString(model.JobLabel)
		sb.WriteString("*")
		sb.WriteString(job)
		sb.WriteString("*")
	}
	if instance, ok := extractInstance(resourceAttrs); ok {
		sb.WriteString(model.InstanceLabel)
		sb.WriteString("*")
		sb.WriteString(instance)
		sb.WriteString("*")
	}

	return sb.String()
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
		dest.SetSum(newer.Sum() + older.Sum())

		counts := make([]uint64, newer.BucketCounts().Len())
		for i := 0; i < newer.BucketCounts().Len(); i++ {
			counts[i] = newer.BucketCounts().At(i) + older.BucketCounts().At(i)
		}
		dest.BucketCounts().FromRaw(counts)
	} else {
		// use new value if bucket bounds do not match
		dest.SetCount(newer.Count())
		dest.SetSum(newer.Sum())
		dest.BucketCounts().FromRaw(newer.BucketCounts().AsRaw())
	}

	dest.ExplicitBounds().FromRaw(newer.ExplicitBounds().AsRaw())
}

// calculateBucketUpperBound calculates the upper bound for an exponential histogram bucket
func calculateBucketUpperBound(scale, offset int32, index int) float64 {
	// For exponential histograms with base = 2:
	// Upper bound = 2^(scale + offset + index + 1)
	return math.Pow(2, float64(scale+offset+int32(index)+1))
}

// filterBucketsForZeroThreshold filters buckets that fall below the zero threshold
// and returns the filtered buckets and the additional zero count
func filterBucketsForZeroThreshold(offset int32, counts []uint64, scale int32, zeroThreshold float64) (newOffset int32, filteredCounts []uint64, additionalZeroCount uint64) {
	if len(counts) == 0 || zeroThreshold <= 0 {
		return offset, counts, 0
	}

	additionalZeroCount = uint64(0)
	filteredCounts = make([]uint64, 0, len(counts))
	newOffset = offset

	// Find the first bucket whose upper bound is > zeroThreshold
	for i, count := range counts {
		upperBound := calculateBucketUpperBound(scale, offset, i)
		if upperBound > zeroThreshold {
			filteredCounts = append(filteredCounts, counts[i:]...)
			break
		}
		// This bucket's range falls entirely below the zero threshold
		additionalZeroCount += count
		newOffset = offset + int32(i) + 1 // Move offset to next bucket
	}

	// If all buckets were filtered out, return empty buckets
	if len(filteredCounts) == 0 {
		return 0, nil, additionalZeroCount
	}

	return newOffset, filteredCounts, additionalZeroCount
}

func accumulateExponentialHistogramValues(prev, current, dest pmetric.ExponentialHistogramDataPoint) {
	if current.Timestamp().AsTime().Before(prev.Timestamp().AsTime()) {
		dest.SetStartTimestamp(current.StartTimestamp())
		prev.Attributes().CopyTo(dest.Attributes())
		dest.SetTimestamp(prev.Timestamp())
	} else {
		dest.SetStartTimestamp(prev.StartTimestamp())
		current.Attributes().CopyTo(dest.Attributes())
		dest.SetTimestamp(current.Timestamp())
	}

	targetScale := min(current.Scale(), prev.Scale())
	dest.SetScale(targetScale)

	// Determine the new zero threshold (maximum of the two)
	newZeroThreshold := max(prev.ZeroThreshold(), current.ZeroThreshold())
	dest.SetZeroThreshold(newZeroThreshold)

	// Downscale buckets to target scale
	pPosOff, pPosCounts := downscaleBucketSide(prev.Positive().Offset(), prev.Positive().BucketCounts().AsRaw(), prev.Scale(), targetScale)
	pNegOff, pNegCounts := downscaleBucketSide(prev.Negative().Offset(), prev.Negative().BucketCounts().AsRaw(), prev.Scale(), targetScale)
	cPosOff, cPosCounts := downscaleBucketSide(current.Positive().Offset(), current.Positive().BucketCounts().AsRaw(), current.Scale(), targetScale)
	cNegOff, cNegCounts := downscaleBucketSide(current.Negative().Offset(), current.Negative().BucketCounts().AsRaw(), current.Scale(), targetScale)

	// Filter buckets that fall below the new zero threshold
	additionalZeroCount := uint64(0)

	// Filter positive buckets from previous histogram
	if newZeroThreshold > prev.ZeroThreshold() {
		var filteredZeroCount uint64
		pPosOff, pPosCounts, filteredZeroCount = filterBucketsForZeroThreshold(pPosOff, pPosCounts, targetScale, newZeroThreshold)
		additionalZeroCount += filteredZeroCount
	}

	// Filter positive buckets from current histogram
	if newZeroThreshold > current.ZeroThreshold() {
		var filteredZeroCount uint64
		cPosOff, cPosCounts, filteredZeroCount = filterBucketsForZeroThreshold(cPosOff, cPosCounts, targetScale, newZeroThreshold)
		additionalZeroCount += filteredZeroCount
	}

	// Merge the remaining buckets
	mPosOff, mPosCounts := mergeBuckets(pPosOff, pPosCounts, cPosOff, cPosCounts)
	mNegOff, mNegCounts := mergeBuckets(pNegOff, pNegCounts, cNegOff, cNegCounts)

	dest.Positive().SetOffset(mPosOff)
	dest.Positive().BucketCounts().FromRaw(mPosCounts)
	dest.Negative().SetOffset(mNegOff)
	dest.Negative().BucketCounts().FromRaw(mNegCounts)

	// Set zero count including additional counts from filtered buckets
	dest.SetZeroCount(prev.ZeroCount() + current.ZeroCount() + additionalZeroCount)
	dest.SetCount(prev.Count() + current.Count())

	if prev.HasSum() && current.HasSum() {
		dest.SetSum(prev.Sum() + current.Sum())
	}

	switch {
	case prev.HasMin() && current.HasMin():
		dest.SetMin(min(prev.Min(), current.Min()))
	case prev.HasMin():
		dest.SetMin(prev.Min())
	case current.HasMin():
		dest.SetMin(current.Min())
	}

	switch {
	case prev.HasMax() && current.HasMax():
		dest.SetMax(max(prev.Max(), current.Max()))
	case prev.HasMax():
		dest.SetMax(prev.Max())
	case current.HasMax():
		dest.SetMax(current.Max())
	}
}

func downscaleBucketSide(offset int32, counts []uint64, fromScale, targetScale int32) (int32, []uint64) {
	if len(counts) == 0 || fromScale <= targetScale {
		return offset, counts
	}
	shift := fromScale - targetScale
	factor := int32(1) << shift

	first := offset
	last := offset + int32(len(counts)) - 1
	newOffset := floorDivInt32(first, factor)
	newLast := floorDivInt32(last, factor)
	newLen := int(newLast - newOffset + 1)
	for i := range counts {
		k := offset + int32(i)
		nk := floorDivInt32(k, factor)
		if k%factor == 0 {
			counts[nk-newOffset] = counts[i]
		} else {
			counts[nk-newOffset] += counts[i]
		}
	}
	return newOffset, counts[:newLen]
}

func mergeBuckets(offsetA int32, countsA []uint64, offsetB int32, countsB []uint64) (int32, []uint64) {
	if len(countsA) == 0 && len(countsB) == 0 {
		return 0, nil
	}
	if len(countsA) == 0 {
		return offsetB, countsB
	}
	if len(countsB) == 0 {
		return offsetA, countsA
	}
	minOffset := min(offsetB, offsetA)
	lastA := offsetA + int32(len(countsA)) - 1
	lastB := offsetB + int32(len(countsB)) - 1
	maxLast := max(lastB, lastA)
	newBucketLen := int(maxLast - minOffset + 1)
	newBucketCount := make([]uint64, newBucketLen)
	for i := range countsA {
		k := offsetA + int32(i)
		newBucketCount[k-minOffset] += countsA[i]
	}
	for i := range countsB {
		k := offsetB + int32(i)
		newBucketCount[k-minOffset] += countsB[i]
	}
	return minOffset, newBucketCount
}

func floorDivInt32(a, b int32) int32 {
	if b <= 0 {
		return 0
	}
	if a >= 0 {
		return a / b
	}
	return -(((-a) + b - 1) / b)
}
