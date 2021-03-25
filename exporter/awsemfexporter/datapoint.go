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

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws"
)

var deltaMetricCalculator = aws.NewFloat64DeltaCalculator()
var summaryMetricCalculator = aws.NewMetricCalculator(calculateSummaryDelta)

func calculateSummaryDelta(prev *aws.MetricValue, val interface{}, timestampMs time.Time) (interface{}, bool) {
	metricEntry := val.(summaryMetricEntry)
	summaryDelta := metricEntry.sum
	countDelta := metricEntry.count
	if prev != nil {
		prevSummaryEntry := prev.RawValue.(summaryMetricEntry)
		summaryDelta = summaryDelta - prevSummaryEntry.sum
		countDelta = countDelta - prevSummaryEntry.count
	}
	return summaryMetricEntry{summaryDelta, countDelta}, true
}

// DataPoint represents a processed metric data point
type DataPoint struct {
	Value       interface{}
	Labels      map[string]string
	TimestampMs int64
}

// DataPoints is a wrapper interface for:
// 	- pdata.IntDataPointSlice
// 	- pdata.DoubleDataPointSlice
// 	- pdata.IntHistogramDataPointSlice
// 	- pdata.HistogramDataPointSlice
//  - pdata.SummaryDataPointSlice
type DataPoints interface {
	Len() int
	// NOTE: At() is an expensive call as it calculates the metric's value
	At(i int) DataPoint
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

// IntDataPointSlice is a wrapper for pdata.IntDataPointSlice
type IntDataPointSlice struct {
	instrumentationLibraryName string
	deltaMetricMetadata
	pdata.IntDataPointSlice
}

// DoubleDataPointSlice is a wrapper for pdata.DoubleDataPointSlice
type DoubleDataPointSlice struct {
	instrumentationLibraryName string
	deltaMetricMetadata
	pdata.DoubleDataPointSlice
}

// HistogramDataPointSlice is a wrapper for pdata.HistogramDataPointSlice
type HistogramDataPointSlice struct {
	instrumentationLibraryName string
	pdata.HistogramDataPointSlice
}

// SummaryDataPointSlice is a wrapper for pdata.SummaryDataPointSlice
type SummaryDataPointSlice struct {
	instrumentationLibraryName string
	deltaMetricMetadata
	pdata.SummaryDataPointSlice
}

type summaryMetricEntry struct {
	sum   float64
	count uint64
}

// At retrieves the IntDataPoint at the given index and performs rate/delta calculation if necessary.
func (dps IntDataPointSlice) At(i int) DataPoint {
	metric := dps.IntDataPointSlice.At(i)
	timestampMs := unixNanoToMilliseconds(metric.Timestamp())
	labels := createLabels(metric.LabelsMap(), dps.instrumentationLibraryName)

	var metricVal float64
	metricVal = float64(metric.Value())
	if dps.adjustToDelta {
		deltaVal, _ := deltaMetricCalculator.Calculate(dps.metricName, mergeLabels(dps.deltaMetricMetadata, labels),
			metricVal, metric.Timestamp().AsTime())
		metricVal = deltaVal.(float64)
	}

	return DataPoint{
		Value:       metricVal,
		Labels:      labels,
		TimestampMs: timestampMs,
	}
}

// At retrieves the DoubleDataPoint at the given index and performs rate/delta calculation if necessary.
func (dps DoubleDataPointSlice) At(i int) DataPoint {
	metric := dps.DoubleDataPointSlice.At(i)
	labels := createLabels(metric.LabelsMap(), dps.instrumentationLibraryName)
	timestampMs := unixNanoToMilliseconds(metric.Timestamp())

	var metricVal float64
	metricVal = metric.Value()
	if dps.adjustToDelta {
		deltaVal, _ := deltaMetricCalculator.Calculate(dps.metricName, mergeLabels(dps.deltaMetricMetadata, labels),
			metricVal, metric.Timestamp().AsTime())
		metricVal = deltaVal.(float64)
	}

	return DataPoint{
		Value:       metricVal,
		Labels:      labels,
		TimestampMs: timestampMs,
	}
}

// At retrieves the HistogramDataPoint at the given index.
func (dps HistogramDataPointSlice) At(i int) DataPoint {
	metric := dps.HistogramDataPointSlice.At(i)
	labels := createLabels(metric.LabelsMap(), dps.instrumentationLibraryName)
	timestamp := unixNanoToMilliseconds(metric.Timestamp())

	return DataPoint{
		Value: &CWMetricStats{
			Count: metric.Count(),
			Sum:   metric.Sum(),
		},
		Labels:      labels,
		TimestampMs: timestamp,
	}
}

// At retrieves the SummaryDataPoint at the given index.
func (dps SummaryDataPointSlice) At(i int) DataPoint {
	metric := dps.SummaryDataPointSlice.At(i)
	labels := createLabels(metric.LabelsMap(), dps.instrumentationLibraryName)
	timestampMs := unixNanoToMilliseconds(metric.Timestamp())

	sum := metric.Sum()
	count := metric.Count()
	if dps.adjustToDelta {
		delta, _ := summaryMetricCalculator.Calculate(dps.metricName, mergeLabels(dps.deltaMetricMetadata, labels),
			summaryMetricEntry{metric.Sum(), metric.Count()}, metric.Timestamp().AsTime())
		summaryMetricDelta := delta.(summaryMetricEntry)
		sum = summaryMetricDelta.sum
		count = summaryMetricDelta.count
	}

	metricVal := &CWMetricStats{
		Count: count,
		Sum:   sum,
	}
	if quantileValues := metric.QuantileValues(); quantileValues.Len() > 0 {
		metricVal.Min = quantileValues.At(0).Value()
		metricVal.Max = quantileValues.At(quantileValues.Len() - 1).Value()
	}

	return DataPoint{
		Value:       metricVal,
		Labels:      labels,
		TimestampMs: timestampMs,
	}
}

// createLabels converts OTel StringMap labels to a map
// and optionally adds in the OTel instrumentation library name
func createLabels(labelsMap pdata.StringMap, instrLibName string) map[string]string {
	labels := make(map[string]string, labelsMap.Len()+1)
	labelsMap.ForEach(func(k, v string) {
		labels[k] = v
	})

	// Add OTel instrumentation lib name as an additional label if it is defined
	if instrLibName != noInstrumentationLibraryName {
		labels[oTellibDimensionKey] = instrLibName
	}

	return labels
}

// getDataPoints retrieves data points from OT Metric.
func getDataPoints(pmd *pdata.Metric, metadata CWMetricMetadata, logger *zap.Logger) (dps DataPoints) {
	if pmd == nil {
		return
	}

	adjusterMetadata := deltaMetricMetadata{
		false,
		pmd.Name(),
		metadata.TimestampMs,
		metadata.Namespace,
		metadata.LogGroup,
		metadata.LogStream,
	}

	switch pmd.DataType() {
	case pdata.MetricDataTypeIntGauge:
		metric := pmd.IntGauge()
		dps = IntDataPointSlice{
			metadata.InstrumentationLibraryName,
			adjusterMetadata,
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeDoubleGauge:
		metric := pmd.DoubleGauge()
		dps = DoubleDataPointSlice{
			metadata.InstrumentationLibraryName,
			adjusterMetadata,
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeIntSum:
		metric := pmd.IntSum()
		adjusterMetadata.adjustToDelta = metric.AggregationTemporality() == pdata.AggregationTemporalityCumulative
		dps = IntDataPointSlice{
			metadata.InstrumentationLibraryName,
			adjusterMetadata,
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeDoubleSum:
		metric := pmd.DoubleSum()
		adjusterMetadata.adjustToDelta = metric.AggregationTemporality() == pdata.AggregationTemporalityCumulative
		dps = DoubleDataPointSlice{
			metadata.InstrumentationLibraryName,
			adjusterMetadata,
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeHistogram:
		metric := pmd.Histogram()
		dps = HistogramDataPointSlice{
			metadata.InstrumentationLibraryName,
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeSummary:
		metric := pmd.Summary()
		adjusterMetadata.adjustToDelta = true
		dps = SummaryDataPointSlice{
			metadata.InstrumentationLibraryName,
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
