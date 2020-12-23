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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/mapwithexpiry"
)

const (
	CleanInterval = 5 * time.Minute
	MinTimeDiff   = 50 * time.Millisecond // We assume 50 milli-seconds is the minimal gap between two collected data sample to be valid to calculate delta
)

var currentState = mapwithexpiry.NewMapWithExpiry(CleanInterval)

// DataPoint represents a processed metric data point
type DataPoint struct {
	Value     interface{}
	Labels    map[string]string
	Timestamp int64
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
	rateKeyParams      map[string]string
	timestamp          int64
}

// rateState stores a metric's value
type rateState struct {
	value     interface{}
	timestamp int64
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
	timestamp := unixNanoToMilliseconds(metric.Timestamp())

	var metricVal interface{}
	metricVal = metric.Value()
	if dps.needsCalculateRate {
		rateKey := createMetricKey(labels, dps.rateKeyParams)
		rateTS := dps.timestamp
		if timestamp > 0 {
			// Use metric timestamp if available
			rateTS = timestamp
		}
		metricVal = calculateRate(rateKey, metricVal, rateTS)
	}

	return DataPoint{
		Value:     metricVal,
		Labels:    labels,
		Timestamp: timestamp,
	}
}

// At retrieves the DoubleDataPoint at the given index and performs rate calculation if necessary.
func (dps DoubleDataPointSlice) At(i int) DataPoint {
	metric := dps.DoubleDataPointSlice.At(i)
	labels := createLabels(metric.LabelsMap(), dps.instrumentationLibraryName)
	timestamp := unixNanoToMilliseconds(metric.Timestamp())

	var metricVal interface{}
	metricVal = metric.Value()
	if dps.needsCalculateRate {
		rateKey := createMetricKey(labels, dps.rateKeyParams)
		rateTS := dps.timestamp
		if timestamp > 0 {
			// Use metric timestamp if available
			rateTS = timestamp
		}
		metricVal = calculateRate(rateKey, metricVal, rateTS)
	}

	return DataPoint{
		Value:     metricVal,
		Labels:    labels,
		Timestamp: timestamp,
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
		Labels:    labels,
		Timestamp: timestamp,
	}
}

// At retrieves the DoubleSummaryDataPoint at the given index.
func (dps DoubleSummaryDataPointSlice) At(i int) DataPoint {
	metric := dps.DoubleSummaryDataPointSlice.At(i)
	labels := createLabels(metric.LabelsMap(), dps.instrumentationLibraryName)
	timestamp := unixNanoToMilliseconds(metric.Timestamp())

	metricVal := &CWMetricStats{
		Count: metric.Count(),
		Sum:   metric.Sum(),
	}
	if quantileValues := metric.QuantileValues(); quantileValues.Len() > 0 {
		metricVal.Min = quantileValues.At(0).Value()
		metricVal.Max = quantileValues.At(quantileValues.Len() - 1).Value()
	}

	return DataPoint{
		Value:     metricVal,
		Labels:    labels,
		Timestamp: timestamp,
	}
}

// createLabels converts OTel StringMap labels to a map and optionally adds in the
// OTel instrumentation library name
func createLabels(labelsMap pdata.StringMap, instrLibName string) map[string]string {
	labels := make(map[string]string, labelsMap.Len()+1)
	labelsMap.ForEach(func(k, v string) {
		labels[k] = v
	})

	// Add OTel instrumentation lib name as an additional label if it is defined
	if instrLibName != noInstrumentationLibraryName {
		labels[OTellibDimensionKey] = instrLibName
	}

	return labels
}

// calculateRate calculates the metric value's rate of change using valDelta / timeDelta.
func calculateRate(metricKey string, val interface{}, timestamp int64) (metricRate interface{}) {
	// get previous Metric content from map. Need to lock the map until set the new state
	currentState.Lock()
	if state, ok := currentState.Get(metricKey); ok {
		prevStats := state.(*rateState)
		deltaTime := timestamp - prevStats.timestamp
		var deltaVal interface{}

		if _, ok := val.(float64); ok {
			if _, ok := prevStats.value.(int64); ok {
				deltaVal = val.(float64) - float64(prevStats.value.(int64))
			} else {
				deltaVal = val.(float64) - prevStats.value.(float64)
			}
			if deltaTime > MinTimeDiff.Milliseconds() && deltaVal.(float64) >= 0 {
				metricRate = deltaVal.(float64) * 1e3 / float64(deltaTime)
			}
		} else {
			if _, ok := prevStats.value.(float64); ok {
				deltaVal = val.(int64) - int64(prevStats.value.(float64))
			} else {
				deltaVal = val.(int64) - prevStats.value.(int64)
			}
			if deltaTime > MinTimeDiff.Milliseconds() && deltaVal.(int64) >= 0 {
				metricRate = deltaVal.(int64) * 1e3 / deltaTime
			}
		}
	}
	content := &rateState{
		value:     val,
		timestamp: timestamp,
	}
	currentState.Set(metricKey, content)
	currentState.Unlock()
	if metricRate == nil {
		if _, ok := val.(float64); ok {
			metricRate = float64(0)
		} else {
			metricRate = int64(0)
		}
	}
	return metricRate
}

// getDataPoints retrieves data points from OT Metric.
func getDataPoints(pmd *pdata.Metric, metadata CWMetricMetadata, logger *zap.Logger) (dps DataPoints) {
	if pmd == nil {
		return
	}

	rateKeyParams := map[string]string{
		(namespaceKey):  metadata.Namespace,
		(metricNameKey): pmd.Name(),
		(logGroupKey):   metadata.LogGroup,
		(logStreamKey):  metadata.LogStream,
	}

	switch pmd.DataType() {
	case pdata.MetricDataTypeIntGauge:
		metric := pmd.IntGauge()
		dps = IntDataPointSlice{
			metadata.InstrumentationLibraryName,
			rateCalculationMetadata{
				false,
				rateKeyParams,
				metadata.Timestamp,
			},
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeDoubleGauge:
		metric := pmd.DoubleGauge()
		dps = DoubleDataPointSlice{
			metadata.InstrumentationLibraryName,
			rateCalculationMetadata{
				false,
				rateKeyParams,
				metadata.Timestamp,
			},
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeIntSum:
		metric := pmd.IntSum()
		dps = IntDataPointSlice{
			metadata.InstrumentationLibraryName,
			rateCalculationMetadata{
				metric.AggregationTemporality() == pdata.AggregationTemporalityCumulative,
				rateKeyParams,
				metadata.Timestamp,
			},
			metric.DataPoints(),
		}
	case pdata.MetricDataTypeDoubleSum:
		metric := pmd.DoubleSum()
		dps = DoubleDataPointSlice{
			metadata.InstrumentationLibraryName,
			rateCalculationMetadata{
				metric.AggregationTemporality() == pdata.AggregationTemporalityCumulative,
				rateKeyParams,
				metadata.Timestamp,
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
