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
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws"
)

var rateMetricCalculator = newFloat64RateCalculator()

func newFloat64RateCalculator() aws.MetricCalculator {
	return aws.NewMetricCalculator(func(prev *aws.MetricValue, val interface{}, timestamp time.Time) (interface{}, bool) {
		if prev != nil {
			deltaTimestampMs := timestamp.Sub(prev.Timestamp).Milliseconds()
			deltaValue := val.(float64) - prev.RawValue.(float64)
			if deltaTimestampMs > 50*time.Millisecond.Milliseconds() && deltaValue >= 0 {
				return deltaValue * 1e3 / float64(deltaTimestampMs), true
			}
		}
		return float64(0), true
	})
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
// 	- pdata.DoubleHistogramDataPointSlice
//  - pdata.DoubleSummaryDataPointSlice
type DataPoints interface {
	Len() int
	// NOTE: At() is an expensive call as it calculates the metric's value
	At(i int) DataPoint
}

// rateCalculationMetadata contains the metadata required to perform rate calculation
type rateCalculationMetadata struct {
	needsCalculateRate bool
	rateKeyParams      rateKeyParams
	timestampMs        int64
}

type rateKeyParams struct {
	namespaceKey  string
	metricNameKey string
	logGroupKey   string
	logStreamKey  string
	timestampKey  string
	labels        attribute.Distinct
}

// IntDataPointSlice is a wrapper for pdata.IntDataPointSlice
type IntDataPointSlice struct {
	instrumentationLibraryName string
	rateCalculationMetadata
	pdata.IntDataPointSlice
}

// DoubleDataPointSlice is a wrapper for pdata.DoubleDataPointSlice
type DoubleDataPointSlice struct {
	instrumentationLibraryName string
	rateCalculationMetadata
	pdata.DoubleDataPointSlice
}

// DoubleHistogramDataPointSlice is a wrapper for pdata.DoubleHistogramDataPointSlice
type DoubleHistogramDataPointSlice struct {
	instrumentationLibraryName string
	pdata.DoubleHistogramDataPointSlice
}

// DoubleSummaryDataPointSlice is a wrapper for pdata.DoubleSummaryDataPointSlice
type DoubleSummaryDataPointSlice struct {
	instrumentationLibraryName string
	pdata.DoubleSummaryDataPointSlice
}

// At retrieves the IntDataPoint at the given index and performs rate calculation if necessary.
func (dps IntDataPointSlice) At(i int) DataPoint {
	metric := dps.IntDataPointSlice.At(i)
	labels := createLabels(metric.LabelsMap(), dps.instrumentationLibraryName)

	timestampMs := unixNanoToMilliseconds(metric.Timestamp())
	rateTimestamp := metric.Timestamp().AsTime()
	if timestampMs == 0 {
		rateTimestamp = time.Unix(0, dps.timestampMs*int64(time.Millisecond))
	}
	var metricVal float64
	metricVal = float64(metric.Value())

	if dps.needsCalculateRate {
		rateVal, _ := rateMetricCalculator.Calculate(dps.rateKeyParams.metricNameKey, labels,
			metricVal, rateTimestamp)
		metricVal = rateVal.(float64)
	}

	return DataPoint{
		Value:       metricVal,
		Labels:      labels,
		TimestampMs: timestampMs,
	}
}

// At retrieves the DoubleDataPoint at the given index and performs rate calculation if necessary.
func (dps DoubleDataPointSlice) At(i int) DataPoint {
	metric := dps.DoubleDataPointSlice.At(i)
	labels := createLabels(metric.LabelsMap(), dps.instrumentationLibraryName)

	timestampMs := unixNanoToMilliseconds(metric.Timestamp())
	rateTimestamp := metric.Timestamp().AsTime()
	if timestampMs == 0 {
		rateTimestamp = time.Unix(0, dps.timestampMs*int64(time.Millisecond))
	}
	metricVal := metric.Value()

	if dps.needsCalculateRate {
		rateVal, _ := rateMetricCalculator.Calculate(dps.rateKeyParams.metricNameKey, labels,
			metricVal, rateTimestamp)
		metricVal = rateVal.(float64)
	}

	return DataPoint{
		Value:       metricVal,
		Labels:      labels,
		TimestampMs: timestampMs,
	}
}

// At retrieves the DoubleHistogramDataPoint at the given index.
func (dps DoubleHistogramDataPointSlice) At(i int) DataPoint {
	metric := dps.DoubleHistogramDataPointSlice.At(i)
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

// At retrieves the DoubleSummaryDataPoint at the given index.
func (dps DoubleSummaryDataPointSlice) At(i int) DataPoint {
	metric := dps.DoubleSummaryDataPointSlice.At(i)
	labels := createLabels(metric.LabelsMap(), dps.instrumentationLibraryName)
	timestampMs := unixNanoToMilliseconds(metric.Timestamp())

	metricVal := &CWMetricStats{
		Count: metric.Count(),
		Sum:   metric.Sum(),
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

// getSortedLabels converts OTel StringMap labels to sorted labels as attribute.Distinct
func getSortedLabels(labels map[string]string) attribute.Distinct {
	var kvs []attribute.KeyValue
	var sortable attribute.Sortable
	for k, v := range labels {
		kvs = append(kvs, attribute.String(k, v))
	}
	set := attribute.NewSetWithSortable(kvs, &sortable)

	return set.Equivalent()
}

// getDataPoints retrieves data points from OT Metric.
func getDataPoints(pmd *pdata.Metric, metadata CWMetricMetadata, logger *zap.Logger) (dps DataPoints) {
	if pmd == nil {
		return
	}

	rateKeys := rateKeyParams{
		namespaceKey:  metadata.Namespace,
		metricNameKey: pmd.Name(),
		logGroupKey:   metadata.LogGroup,
		logStreamKey:  metadata.LogStream,
	}

	switch pmd.DataType() {
	case pdata.MetricDataTypeIntGauge:
		metric := pmd.IntGauge()
		dps = IntDataPointSlice{
			metadata.InstrumentationLibraryName,
			rateCalculationMetadata{
				false,
				rateKeys,
				metadata.TimestampMs,
			},
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeDoubleGauge:
		metric := pmd.DoubleGauge()
		dps = DoubleDataPointSlice{
			metadata.InstrumentationLibraryName,
			rateCalculationMetadata{
				false,
				rateKeys,
				metadata.TimestampMs,
			},
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeIntSum:
		metric := pmd.IntSum()
		dps = IntDataPointSlice{
			metadata.InstrumentationLibraryName,
			rateCalculationMetadata{
				metric.AggregationTemporality() == pdata.AggregationTemporalityCumulative,
				rateKeys,
				metadata.TimestampMs,
			},
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeDoubleSum:
		metric := pmd.DoubleSum()
		dps = DoubleDataPointSlice{
			metadata.InstrumentationLibraryName,
			rateCalculationMetadata{
				metric.AggregationTemporality() == pdata.AggregationTemporalityCumulative,
				rateKeys,
				metadata.TimestampMs,
			},
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeDoubleHistogram:
		metric := pmd.DoubleHistogram()
		dps = DoubleHistogramDataPointSlice{
			metadata.InstrumentationLibraryName,
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeDoubleSummary:
		metric := pmd.DoubleSummary()
		dps = DoubleSummaryDataPointSlice{
			metadata.InstrumentationLibraryName,
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
