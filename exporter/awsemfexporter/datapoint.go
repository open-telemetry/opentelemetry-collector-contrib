// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"

	aws "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
)

const (
	summaryCountSuffix = "_count"
	summarySumSuffix   = "_sum"
)

type emfCalculators struct {
	delta   aws.MetricCalculator
	summary aws.MetricCalculator
}

func calculateSummaryDelta(prev *aws.MetricValue, val any, _ time.Time) (any, bool) {
	metricEntry := val.(summaryMetricEntry)
	summaryDelta := metricEntry.sum
	countDelta := metricEntry.count
	if prev != nil {
		prevSummaryEntry := prev.RawValue.(summaryMetricEntry)
		summaryDelta = metricEntry.sum - prevSummaryEntry.sum
		countDelta = metricEntry.count - prevSummaryEntry.count
	} else {
		return summaryMetricEntry{summaryDelta, countDelta}, false
	}
	return summaryMetricEntry{summaryDelta, countDelta}, true
}

// dataPoint represents a processed metric data point
type dataPoint struct {
	name        string
	value       any
	labels      map[string]string
	timestampMs int64
}

// dataPoints is a wrapper interface for:
//   - pmetric.NumberDataPointSlice
//   - pmetric.HistogramDataPointSlice
//   - pmetric.SummaryDataPointSlice
type dataPoints interface {
	Len() int
	// CalculateDeltaDatapoints calculates the delta datapoint from the DataPointSlice at i-th index
	// for some type (Counter, Summary)
	// dataPoint: the adjusted data point
	// retained: indicates whether the data point is valid for further process
	// NOTE: It is an expensive call as it calculates the metric value.
	CalculateDeltaDatapoints(i int, instrumentationScopeName string, detailedMetrics bool, calculators *emfCalculators) (dataPoint []dataPoint, retained bool)
	// IsStaleNaNInf returns true if metric value has NoRecordedValue flag set or if any metric value contains a NaN or Inf.
	// When return value is true, IsStaleNaNInf also returns the attributes attached to the metric which can be used for
	// logging purposes.
	IsStaleNaNInf(i int) (bool, pcommon.Map)
}

// deltaMetricMetadata contains the metadata required to perform rate/delta calculation
type deltaMetricMetadata struct {
	adjustToDelta              bool
	retainInitialValueForDelta bool
	metricName                 string
	namespace                  string
	logGroup                   string
	logStream                  string
}

// numberDataPointSlice is a wrapper for pmetric.NumberDataPointSlice
type numberDataPointSlice struct {
	deltaMetricMetadata
	pmetric.NumberDataPointSlice
}

// histogramDataPointSlice is a wrapper for pmetric.HistogramDataPointSlice
type histogramDataPointSlice struct {
	// Todo:(khanhntd) Calculate delta value for count and sum value with histogram
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18245
	deltaMetricMetadata
	pmetric.HistogramDataPointSlice
}

type exponentialHistogramDataPointSlice struct {
	// TODO: Calculate delta value for count and sum value with exponential histogram
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18245
	deltaMetricMetadata
	pmetric.ExponentialHistogramDataPointSlice
}

// summaryDataPointSlice is a wrapper for pmetric.SummaryDataPointSlice
type summaryDataPointSlice struct {
	deltaMetricMetadata
	pmetric.SummaryDataPointSlice
}

type summaryMetricEntry struct {
	sum   float64
	count uint64
}

// dataPointSplit is a structure used to manage segments of data points split from a histogram.
// It is not safe for concurrent use.
type dataPointSplit struct {
	cWMetricHistogram *cWMetricHistogram
	length            int
	capacity          int
}

func (split *dataPointSplit) isFull() bool {
	return split.length >= split.capacity
}

func (split *dataPointSplit) setMax(maxVal float64) {
	split.cWMetricHistogram.Max = maxVal
}

func (split *dataPointSplit) setMin(minVal float64) {
	split.cWMetricHistogram.Min = minVal
}

func (split *dataPointSplit) appendMetricData(metricVal float64, count uint64) {
	split.cWMetricHistogram.Values = append(split.cWMetricHistogram.Values, metricVal)
	split.cWMetricHistogram.Counts = append(split.cWMetricHistogram.Counts, float64(count))
	split.length++
	split.cWMetricHistogram.Count += count
}

// CalculateDeltaDatapoints retrieves the NumberDataPoint at the given index and performs rate/delta calculation if necessary.
func (dps numberDataPointSlice) CalculateDeltaDatapoints(i int, instrumentationScopeName string, _ bool, calculators *emfCalculators) ([]dataPoint, bool) {
	metric := dps.NumberDataPointSlice.At(i)
	labels := createLabels(metric.Attributes(), instrumentationScopeName)
	timestampMs := unixNanoToMilliseconds(metric.Timestamp())

	var metricVal float64
	switch metric.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		metricVal = metric.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		metricVal = float64(metric.IntValue())
	}

	retained := true

	if dps.adjustToDelta {
		var deltaVal any
		mKey := aws.NewKey(dps.deltaMetricMetadata, labels)
		deltaVal, retained = calculators.delta.Calculate(mKey, metricVal, metric.Timestamp().AsTime())

		// If a delta to the previous data point could not be computed use the current metric value instead
		if !retained && dps.retainInitialValueForDelta {
			retained = true
			deltaVal = metricVal
		}

		if !retained {
			return nil, retained
		}
		// It should not happen in practice that the previous metric value is smaller than the current one.
		// If it happens, we assume that the metric is reset for some reason.
		if deltaVal.(float64) >= 0 {
			metricVal = deltaVal.(float64)
		}
	}

	return []dataPoint{{name: dps.metricName, value: metricVal, labels: labels, timestampMs: timestampMs}}, retained
}

func (dps numberDataPointSlice) IsStaleNaNInf(i int) (bool, pcommon.Map) {
	metric := dps.NumberDataPointSlice.At(i)
	if metric.Flags().NoRecordedValue() {
		return true, metric.Attributes()
	}
	if metric.ValueType() == pmetric.NumberDataPointValueTypeDouble {
		return math.IsNaN(metric.DoubleValue()) || math.IsInf(metric.DoubleValue(), 0), metric.Attributes()
	}
	return false, pcommon.Map{}
}

// CalculateDeltaDatapoints retrieves the HistogramDataPoint at the given index.
func (dps histogramDataPointSlice) CalculateDeltaDatapoints(i int, instrumentationScopeName string, _ bool, _ *emfCalculators) ([]dataPoint, bool) {
	metric := dps.HistogramDataPointSlice.At(i)
	labels := createLabels(metric.Attributes(), instrumentationScopeName)
	timestamp := unixNanoToMilliseconds(metric.Timestamp())

	return []dataPoint{{
		name: dps.metricName,
		value: &cWMetricStats{
			Count: metric.Count(),
			Sum:   metric.Sum(),
			Max:   metric.Max(),
			Min:   metric.Min(),
		},
		labels:      labels,
		timestampMs: timestamp,
	}}, true
}

func (dps histogramDataPointSlice) IsStaleNaNInf(i int) (bool, pcommon.Map) {
	metric := dps.HistogramDataPointSlice.At(i)
	if metric.Flags().NoRecordedValue() {
		return true, metric.Attributes()
	}
	if math.IsNaN(metric.Max()) || math.IsNaN(metric.Sum()) ||
		math.IsNaN(metric.Min()) || math.IsInf(metric.Max(), 0) ||
		math.IsInf(metric.Sum(), 0) || math.IsInf(metric.Min(), 0) {
		return true, metric.Attributes()
	}
	return false, pcommon.Map{}
}

// CalculateDeltaDatapoints retrieves the ExponentialHistogramDataPoint at the given index.
// As CloudWatch EMF logs allows in maximum of 100 target members, the exponential histogram metric are split into multiple data points as needed,
// each containing a maximum of 100 buckets, to comply with CloudWatch EMF log constraints.
// Note that the number of values and counts in each split may not be less than splitThreshold as we are only adding non-zero bucket counts.
//
// For each split data point:
// - Min and Max values are recalculated based on the bucket boundary within that specific split.
// - Sum is only assigned to the first split to ensure the total sum of the datapoints after aggregation is correct.
// - Count is accumulated based on the bucket counts within each split.
func (dps exponentialHistogramDataPointSlice) CalculateDeltaDatapoints(idx int, instrumentationScopeName string, _ bool, _ *emfCalculators) ([]dataPoint, bool) {
	metric := dps.ExponentialHistogramDataPointSlice.At(idx)

	const splitThreshold = 100
	currentBucketIndex := 0
	currentPositiveIndex := metric.Positive().BucketCounts().Len() - 1
	currentZeroIndex := 0
	currentNegativeIndex := 0
	var datapoints []dataPoint
	totalBucketLen := metric.Positive().BucketCounts().Len() + metric.Negative().BucketCounts().Len()
	if metric.ZeroCount() > 0 {
		totalBucketLen++
	}

	for currentBucketIndex < totalBucketLen {
		// Create a new dataPointSplit with a capacity of up to splitThreshold buckets
		capacity := min(splitThreshold, totalBucketLen-currentBucketIndex)

		sum := 0.0
		// Only assign `Sum` if this is the first split to make sure the total sum of the datapoints after aggregation is correct.
		if currentBucketIndex == 0 {
			sum = metric.Sum()
		}

		split := dataPointSplit{
			cWMetricHistogram: &cWMetricHistogram{
				Values: []float64{},
				Counts: []float64{},
				Max:    metric.Max(),
				Min:    metric.Min(),
				Count:  0,
				Sum:    sum,
			},
			length:   0,
			capacity: capacity,
		}

		// Set collect values from positive buckets and save into split.
		currentBucketIndex, currentPositiveIndex = collectDatapointsWithPositiveBuckets(&split, metric, currentBucketIndex, currentPositiveIndex)
		// Set collect values from zero buckets and save into split.
		currentBucketIndex, currentZeroIndex = collectDatapointsWithZeroBucket(&split, metric, currentBucketIndex, currentZeroIndex)
		// Set collect values from negative buckets and save into split.
		currentBucketIndex, currentNegativeIndex = collectDatapointsWithNegativeBuckets(&split, metric, currentBucketIndex, currentNegativeIndex)

		if split.length > 0 {
			// Add the current split to the datapoints list
			datapoints = append(datapoints, dataPoint{
				name:        dps.metricName,
				value:       split.cWMetricHistogram,
				labels:      createLabels(metric.Attributes(), instrumentationScopeName),
				timestampMs: unixNanoToMilliseconds(metric.Timestamp()),
			})
		}
	}

	if len(datapoints) == 0 {
		return []dataPoint{{
			name: dps.metricName,
			value: &cWMetricHistogram{
				Values: []float64{},
				Counts: []float64{},
				Count:  metric.Count(),
				Sum:    metric.Sum(),
				Max:    metric.Max(),
				Min:    metric.Min(),
			},
			labels:      createLabels(metric.Attributes(), instrumentationScopeName),
			timestampMs: unixNanoToMilliseconds(metric.Timestamp()),
		}}, true
	}

	// Override the min and max values of the first and last splits with the raw data of the metric.
	datapoints[0].value.(*cWMetricHistogram).Max = metric.Max()
	datapoints[len(datapoints)-1].value.(*cWMetricHistogram).Min = metric.Min()

	return datapoints, true
}

func collectDatapointsWithPositiveBuckets(split *dataPointSplit, metric pmetric.ExponentialHistogramDataPoint, currentBucketIndex int, currentPositiveIndex int) (int, int) {
	if split.isFull() || currentPositiveIndex < 0 {
		return currentBucketIndex, currentPositiveIndex
	}

	scale := metric.Scale()
	base := math.Pow(2, math.Pow(2, float64(-scale)))
	positiveBuckets := metric.Positive()
	positiveOffset := positiveBuckets.Offset()
	positiveBucketCounts := positiveBuckets.BucketCounts()
	bucketBegin := 0.0
	bucketEnd := 0.0

	for !split.isFull() && currentPositiveIndex >= 0 {
		index := currentPositiveIndex + int(positiveOffset)
		if bucketEnd == 0 {
			bucketEnd = math.Pow(base, float64(index+1))
		} else {
			bucketEnd = bucketBegin
		}
		bucketBegin = math.Pow(base, float64(index))
		metricVal := (bucketBegin + bucketEnd) / 2
		count := positiveBucketCounts.At(currentPositiveIndex)
		if count > 0 {
			split.appendMetricData(metricVal, count)

			// The value are append from high to low, set Max from the first bucket (highest value) and Min from the last bucket (lowest value)
			if split.length == 1 {
				split.setMax(bucketEnd)
			}
			if split.isFull() {
				split.setMin(bucketBegin)
			}
		}
		currentBucketIndex++
		currentPositiveIndex--
	}

	return currentBucketIndex, currentPositiveIndex
}

func collectDatapointsWithZeroBucket(split *dataPointSplit, metric pmetric.ExponentialHistogramDataPoint, currentBucketIndex int, currentZeroIndex int) (int, int) {
	if metric.ZeroCount() > 0 && !split.isFull() && currentZeroIndex == 0 {
		split.appendMetricData(0, metric.ZeroCount())

		// The value are append from high to low, set Max from the first bucket (highest value) and Min from the last bucket (lowest value)
		if split.length == 1 {
			split.setMax(0)
		}
		if split.isFull() {
			split.setMin(0)
		}
		currentZeroIndex++
		currentBucketIndex++
	}

	return currentBucketIndex, currentZeroIndex
}

func collectDatapointsWithNegativeBuckets(split *dataPointSplit, metric pmetric.ExponentialHistogramDataPoint, currentBucketIndex int, currentNegativeIndex int) (int, int) {
	// According to metrics spec, the value in histogram is expected to be non-negative.
	// https://opentelemetry.io/docs/specs/otel/metrics/api/#histogram
	// However, the negative support is defined in metrics data model.
	// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#exponentialhistogram
	// The negative is also supported but only verified with unit test.
	if split.isFull() || currentNegativeIndex >= metric.Negative().BucketCounts().Len() {
		return currentBucketIndex, currentNegativeIndex
	}

	scale := metric.Scale()
	base := math.Pow(2, math.Pow(2, float64(-scale)))
	negativeBuckets := metric.Negative()
	negativeOffset := negativeBuckets.Offset()
	negativeBucketCounts := negativeBuckets.BucketCounts()
	bucketBegin := 0.0
	bucketEnd := 0.0

	for !split.isFull() && currentNegativeIndex < metric.Negative().BucketCounts().Len() {
		index := currentNegativeIndex + int(negativeOffset)
		if bucketEnd == 0 {
			bucketEnd = -math.Pow(base, float64(index))
		} else {
			bucketEnd = bucketBegin
		}
		bucketBegin = -math.Pow(base, float64(index+1))
		metricVal := (bucketBegin + bucketEnd) / 2
		count := negativeBucketCounts.At(currentNegativeIndex)
		if count > 0 {
			split.appendMetricData(metricVal, count)

			// The value are append from high to low, set Max from the first bucket (highest value) and Min from the last bucket (lowest value)
			if split.length == 1 {
				split.setMax(bucketEnd)
			}
			if split.isFull() {
				split.setMin(bucketBegin)
			}
		}
		currentBucketIndex++
		currentNegativeIndex++
	}

	return currentBucketIndex, currentNegativeIndex
}

func (dps exponentialHistogramDataPointSlice) IsStaleNaNInf(i int) (bool, pcommon.Map) {
	metric := dps.ExponentialHistogramDataPointSlice.At(i)
	if metric.Flags().NoRecordedValue() {
		return true, metric.Attributes()
	}
	if math.IsNaN(metric.Max()) ||
		math.IsNaN(metric.Min()) ||
		math.IsNaN(metric.Sum()) ||
		math.IsInf(metric.Max(), 0) ||
		math.IsInf(metric.Min(), 0) ||
		math.IsInf(metric.Sum(), 0) {
		return true, metric.Attributes()
	}

	return false, pcommon.Map{}
}

// CalculateDeltaDatapoints retrieves the SummaryDataPoint at the given index and perform calculation with sum and count while retain the quantile value.
func (dps summaryDataPointSlice) CalculateDeltaDatapoints(i int, instrumentationScopeName string, detailedMetrics bool, calculators *emfCalculators) ([]dataPoint, bool) {
	metric := dps.SummaryDataPointSlice.At(i)
	labels := createLabels(metric.Attributes(), instrumentationScopeName)
	timestampMs := unixNanoToMilliseconds(metric.Timestamp())

	sum := metric.Sum()
	count := metric.Count()

	retained := true
	datapoints := []dataPoint{}

	if dps.adjustToDelta {
		var delta any
		mKey := aws.NewKey(dps.deltaMetricMetadata, labels)
		delta, retained = calculators.summary.Calculate(mKey, summaryMetricEntry{sum, count}, metric.Timestamp().AsTime())

		// If a delta to the previous data point could not be computed use the current metric value instead
		if !retained && dps.retainInitialValueForDelta {
			retained = true
			delta = summaryMetricEntry{sum, count}
		}

		if !retained {
			return datapoints, retained
		}
		summaryMetricDelta := delta.(summaryMetricEntry)
		sum = summaryMetricDelta.sum
		count = summaryMetricDelta.count
	}

	if detailedMetrics {
		// Instead of sending metrics as a Statistical Set (contains min,max, count, sum), the emfexporter will enrich the
		// values by sending each quantile values as a datapoint (from quantile 0 ... 1)
		values := metric.QuantileValues()
		datapoints = append(datapoints, dataPoint{name: fmt.Sprint(dps.metricName, summarySumSuffix), value: sum, labels: labels, timestampMs: timestampMs})
		datapoints = append(datapoints, dataPoint{name: fmt.Sprint(dps.metricName, summaryCountSuffix), value: count, labels: labels, timestampMs: timestampMs})

		for i := 0; i < values.Len(); i++ {
			cLabels := maps.Clone(labels)
			quantile := values.At(i)
			cLabels["quantile"] = strconv.FormatFloat(quantile.Quantile(), 'g', -1, 64)
			datapoints = append(datapoints, dataPoint{name: dps.metricName, value: quantile.Value(), labels: cLabels, timestampMs: timestampMs})
		}
	} else {
		metricVal := &cWMetricStats{Count: count, Sum: sum}
		if quantileValues := metric.QuantileValues(); quantileValues.Len() > 0 {
			metricVal.Min = quantileValues.At(0).Value()
			metricVal.Max = quantileValues.At(quantileValues.Len() - 1).Value()
		}
		datapoints = append(datapoints, dataPoint{name: dps.metricName, value: metricVal, labels: labels, timestampMs: timestampMs})
	}

	return datapoints, retained
}

func (dps summaryDataPointSlice) IsStaleNaNInf(i int) (bool, pcommon.Map) {
	metric := dps.SummaryDataPointSlice.At(i)
	if metric.Flags().NoRecordedValue() {
		return true, metric.Attributes()
	}
	if math.IsNaN(metric.Sum()) || math.IsInf(metric.Sum(), 0) {
		return true, metric.Attributes()
	}

	values := metric.QuantileValues()
	for i := 0; i < values.Len(); i++ {
		quantile := values.At(i)
		if math.IsNaN(quantile.Value()) || math.IsNaN(quantile.Quantile()) ||
			math.IsInf(quantile.Value(), 0) || math.IsInf(quantile.Quantile(), 0) {
			return true, metric.Attributes()
		}
	}

	return false, metric.Attributes()
}

// createLabels converts OTel AttributesMap attributes to a map
// and optionally adds in the OTel instrumentation library name
func createLabels(attributes pcommon.Map, instrLibName string) map[string]string {
	labels := make(map[string]string, attributes.Len()+1)
	attributes.Range(func(k string, v pcommon.Value) bool {
		labels[k] = v.AsString()
		return true
	})

	// Add OTel instrumentation lib name as an additional label if it is defined
	if instrLibName != "" {
		labels[oTellibDimensionKey] = instrLibName
	}

	return labels
}

// getDataPoints retrieves data points from OT Metric.
func getDataPoints(pmd pmetric.Metric, metadata cWMetricMetadata, logger *zap.Logger) dataPoints {
	metricMetadata := deltaMetricMetadata{
		adjustToDelta:              false,
		retainInitialValueForDelta: metadata.retainInitialValueForDelta,
		metricName:                 pmd.Name(),
		namespace:                  metadata.namespace,
		logGroup:                   metadata.logGroup,
		logStream:                  metadata.logStream,
	}

	var dps dataPoints

	//exhaustive:enforce
	switch pmd.Type() {
	case pmetric.MetricTypeGauge:
		metric := pmd.Gauge()
		dps = numberDataPointSlice{
			metricMetadata,
			metric.DataPoints(),
		}
	case pmetric.MetricTypeSum:
		metric := pmd.Sum()
		metricMetadata.adjustToDelta = metric.AggregationTemporality() == pmetric.AggregationTemporalityCumulative
		dps = numberDataPointSlice{
			metricMetadata,
			metric.DataPoints(),
		}
	case pmetric.MetricTypeHistogram:
		metric := pmd.Histogram()
		dps = histogramDataPointSlice{
			metricMetadata,
			metric.DataPoints(),
		}
	case pmetric.MetricTypeExponentialHistogram:
		metric := pmd.ExponentialHistogram()
		dps = exponentialHistogramDataPointSlice{
			metricMetadata,
			metric.DataPoints(),
		}
	case pmetric.MetricTypeSummary:
		metric := pmd.Summary()
		// For summaries coming from the prometheus receiver, the sum and count are cumulative, whereas for summaries
		// coming from other sources, e.g. SDK, the sum and count are delta by being accumulated and reset periodically.
		// In order to ensure metrics are sent as deltas, we check the receiver attribute (which can be injected by
		// attribute processor) from resource metrics. If it exists, and equals to prometheus, the sum and count will be
		// converted.
		// For more information: https://github.com/open-telemetry/opentelemetry-collector/blob/main/receiver/prometheusreceiver/DESIGN.md#summary
		metricMetadata.adjustToDelta = metadata.receiver == prometheusReceiver
		dps = summaryDataPointSlice{
			metricMetadata,
			metric.DataPoints(),
		}
	default:
		logger.Warn("Unhandled metric data type.",
			zap.String("DataType", pmd.Type().String()),
			zap.String("Name", pmd.Name()),
			zap.String("Unit", pmd.Unit()),
		)
	}

	return dps
}
