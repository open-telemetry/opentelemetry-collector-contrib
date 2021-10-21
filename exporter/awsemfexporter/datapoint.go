// Copyright 2020, OpenTelemetry Authors
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

package awsemfexporter

import (
	"time"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	aws "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
)

var deltaMetricCalculator = aws.NewFloat64DeltaCalculator()
var summaryMetricCalculator = aws.NewMetricCalculator(calculateSummaryDelta)

func calculateSummaryDelta(prev *aws.MetricValue, val interface{}, timestampMs time.Time) (interface{}, bool) {
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
	value       interface{}
	labels      map[string]string
	timestampMs int64
}

// dataPoints is a wrapper interface for:
// 	- pdata.NumberDataPointSlice
// 	- pdata.histogramDataPointSlice
//  - pdata.summaryDataPointSlice
type dataPoints interface {
	Len() int
	// At gets the adjusted datapoint from the DataPointSlice at i-th index.
	// dataPoint: the adjusted data point
	// retained: indicates whether the data point is valid for further process
	// NOTE: It is an expensive call as it calculates the metric value.
	At(i int) (dataPoint dataPoint, retained bool)
}

// deltaMetricMetadata contains the metadata required to perform rate/delta calculation
type deltaMetricMetadata struct {
	adjustToDelta bool
	metricName    string
	timestampMs   int64
	namespace     string
	logGroup      string
	logStream     string
}

func mergeLabels(m deltaMetricMetadata, labels map[string]string) map[string]string {
	result := map[string]string{
		"namespace": m.namespace,
		"logGroup":  m.logGroup,
		"logStream": m.logStream,
	}
	for k, v := range labels {
		result[k] = v
	}
	return result
}

// numberDataPointSlice is a wrapper for pdata.NumberDataPointSlice
type numberDataPointSlice struct {
	instrumentationLibraryName string
	deltaMetricMetadata
	pdata.NumberDataPointSlice
}

// histogramDataPointSlice is a wrapper for pdata.histogramDataPointSlice
type histogramDataPointSlice struct {
	instrumentationLibraryName string
	pdata.HistogramDataPointSlice
}

// summaryDataPointSlice is a wrapper for pdata.summaryDataPointSlice
type summaryDataPointSlice struct {
	instrumentationLibraryName string
	deltaMetricMetadata
	pdata.SummaryDataPointSlice
}

type summaryMetricEntry struct {
	sum   float64
	count uint64
}

// At retrieves the NumberDataPoint at the given index and performs rate/delta calculation if necessary.
func (dps numberDataPointSlice) At(i int) (dataPoint, bool) {
	metric := dps.NumberDataPointSlice.At(i)
	labels := createLabels(metric.Attributes(), dps.instrumentationLibraryName)
	timestampMs := unixNanoToMilliseconds(metric.Timestamp())

	var metricVal float64
	switch metric.Type() {
	case pdata.MetricValueTypeDouble:
		metricVal = metric.DoubleVal()
	case pdata.MetricValueTypeInt:
		metricVal = float64(metric.IntVal())
	}

	retained := true
	if dps.adjustToDelta {
		var deltaVal interface{}
		deltaVal, retained = deltaMetricCalculator.Calculate(dps.metricName, mergeLabels(dps.deltaMetricMetadata, labels),
			metricVal, metric.Timestamp().AsTime())
		if !retained {
			return dataPoint{}, retained
		}
		// It should not happen in practice that the previous metric value is smaller than the current one.
		// If it happens, we assume that the metric is reset for some reason.
		if deltaVal.(float64) >= 0 {
			metricVal = deltaVal.(float64)
		}
	}

	return dataPoint{
		value:       metricVal,
		labels:      labels,
		timestampMs: timestampMs,
	}, retained
}

// At retrieves the HistogramDataPoint at the given index.
func (dps histogramDataPointSlice) At(i int) (dataPoint, bool) {
	metric := dps.HistogramDataPointSlice.At(i)
	labels := createLabels(metric.Attributes(), dps.instrumentationLibraryName)
	timestamp := unixNanoToMilliseconds(metric.Timestamp())

	return dataPoint{
		value: &cWMetricStats{
			Count: metric.Count(),
			Sum:   metric.Sum(),
		},
		labels:      labels,
		timestampMs: timestamp,
	}, true
}

// At retrieves the SummaryDataPoint at the given index.
func (dps summaryDataPointSlice) At(i int) (dataPoint, bool) {
	metric := dps.SummaryDataPointSlice.At(i)
	labels := createLabels(metric.Attributes(), dps.instrumentationLibraryName)
	timestampMs := unixNanoToMilliseconds(metric.Timestamp())

	sum := metric.Sum()
	count := metric.Count()
	retained := true
	if dps.adjustToDelta {
		var delta interface{}
		delta, retained = summaryMetricCalculator.Calculate(dps.metricName, mergeLabels(dps.deltaMetricMetadata, labels),
			summaryMetricEntry{metric.Sum(), metric.Count()}, metric.Timestamp().AsTime())
		if !retained {
			return dataPoint{}, retained
		}
		summaryMetricDelta := delta.(summaryMetricEntry)
		sum = summaryMetricDelta.sum
		count = summaryMetricDelta.count
	}

	metricVal := &cWMetricStats{
		Count: count,
		Sum:   sum,
	}
	if quantileValues := metric.QuantileValues(); quantileValues.Len() > 0 {
		metricVal.Min = quantileValues.At(0).Value()
		metricVal.Max = quantileValues.At(quantileValues.Len() - 1).Value()
	}

	return dataPoint{
		value:       metricVal,
		labels:      labels,
		timestampMs: timestampMs,
	}, retained
}

// createLabels converts OTel AttributesMap attributes to a map
// and optionally adds in the OTel instrumentation library name
func createLabels(attributes pdata.AttributeMap, instrLibName string) map[string]string {
	labels := make(map[string]string, attributes.Len()+1)
	attributes.Range(func(k string, v pdata.AttributeValue) bool {
		labels[k] = v.AsString()
		return true
	})

	// Add OTel instrumentation lib name as an additional label if it is defined
	if instrLibName != noInstrumentationLibraryName {
		labels[oTellibDimensionKey] = instrLibName
	}

	return labels
}

// getDataPoints retrieves data points from OT Metric.
func getDataPoints(pmd *pdata.Metric, metadata cWMetricMetadata, logger *zap.Logger) (dps dataPoints) {
	if pmd == nil {
		return
	}

	adjusterMetadata := deltaMetricMetadata{
		false,
		pmd.Name(),
		metadata.timestampMs,
		metadata.namespace,
		metadata.logGroup,
		metadata.logStream,
	}

	switch pmd.DataType() {
	case pdata.MetricDataTypeGauge:
		metric := pmd.Gauge()
		dps = numberDataPointSlice{
			metadata.instrumentationLibraryName,
			adjusterMetadata,
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeSum:
		metric := pmd.Sum()
		adjusterMetadata.adjustToDelta = metric.AggregationTemporality() == pdata.MetricAggregationTemporalityCumulative
		dps = numberDataPointSlice{
			metadata.instrumentationLibraryName,
			adjusterMetadata,
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeHistogram:
		metric := pmd.Histogram()
		dps = histogramDataPointSlice{
			metadata.instrumentationLibraryName,
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeSummary:
		metric := pmd.Summary()
		// For summaries coming from the prometheus receiver, the sum and count are cumulative, whereas for summaries
		// coming from other sources, e.g. SDK, the sum and count are delta by being accumulated and reset periodically.
		// In order to ensure metrics are sent as deltas, we check the receiver attribute (which can be injected by
		// attribute processor) from resource metrics. If it exists, and equals to prometheus, the sum and count will be
		// converted.
		// For more information: https://github.com/open-telemetry/opentelemetry-collector/blob/main/receiver/prometheusreceiver/DESIGN.md#summary
		adjusterMetadata.adjustToDelta = metadata.receiver == prometheusReceiver
		dps = summaryDataPointSlice{
			metadata.instrumentationLibraryName,
			adjusterMetadata,
			metric.DataPoints(),
		}
	default:
		logger.Warn("Unhandled metric data type.",
			zap.String("DataType", pmd.DataType().String()),
			zap.String("Name", pmd.Name()),
			zap.String("Unit", pmd.Unit()),
		)
	}
	return
}
