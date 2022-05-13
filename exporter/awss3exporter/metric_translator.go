// Copyright 2021, OpenTelemetry Authors
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

package awss3exporter

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	_attributeReceiver            = "receiver"
	_noInstrumentationLibraryName = "Undefined"
	_oTellibDimensionKey          = "OTelLib"
)

// DataPoint represents a processed metric data point
type DataPoint struct {
	Value       interface{}
	Labels      map[string]string
	TimestampMs int64
}

type DataPoints interface {
	Len() int
	At(i int) (dataPoint DataPoint)
}

type NumberDataPointSlice struct {
	instrumentationLibraryName string
	pmetric.NumberDataPointSlice
}

// At retrieves the NumberDataPoint at the given index and performs rate/delta calculation if necessary.
func (dps NumberDataPointSlice) At(i int) DataPoint {
	metric := dps.NumberDataPointSlice.At(i)
	labels := createLabels(metric.Attributes(), dps.instrumentationLibraryName)
	timestampMs := unixNanoToMilliseconds(metric.Timestamp())

	var metricVal float64
	switch metric.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		metricVal = metric.DoubleVal()
	case pmetric.NumberDataPointValueTypeInt:
		metricVal = float64(metric.IntVal())
	}

	return DataPoint{
		Value:       metricVal,
		Labels:      labels,
		TimestampMs: timestampMs,
	}
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
	if instrLibName != _noInstrumentationLibraryName {
		labels[_oTellibDimensionKey] = instrLibName
	}

	return labels
}

type MetricDescriptor struct {
	// metricName is the name of the metric
	metricName string `mapstructure:"metric_name"`
	// unit defines the override value of metric descriptor `unit`
	unit string `mapstructure:"unit"`
	// overwrite set to true means the existing metric descriptor will be overwritten or a new metric descriptor will be created; false means
	// the descriptor will only be configured if empty.
	overwrite bool `mapstructure:"overwrite"`
}

type metricTranslator struct {
	metricDescriptor map[string]MetricDescriptor
}

// unixNanoToMilliseconds converts a timestamp in nanoseconds to milliseconds.
func unixNanoToMilliseconds(timestamp pcommon.Timestamp) int64 {
	return int64(uint64(timestamp) / uint64(time.Millisecond))
}

func newMetricTranslator(config Config) metricTranslator {
	mt := map[string]MetricDescriptor{}
	for _, descriptor := range config.MetricDescriptors {
		mt[descriptor.metricName] = descriptor
	}
	return metricTranslator{
		metricDescriptor: mt,
	}
}

// getDataPoints retrieves data points from OT Metric.
func getDataPoints(pmd *pmetric.Metric, instrumentationLibName string, logger *zap.Logger) (dps DataPoints) {
	if pmd == nil {
		return
	}

	switch pmd.DataType() {
	case pmetric.MetricDataTypeGauge:
		metric := pmd.Gauge()
		dps = NumberDataPointSlice{
			instrumentationLibName,
			metric.DataPoints(),
		}
	case pmetric.MetricDataTypeSum:
		metric := pmd.Sum()
		dps = NumberDataPointSlice{
			instrumentationLibName,
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

// // MetricInfo defines value and unit for OT Metrics
type MetricInfo struct {
	Value interface{}
	Unit  string
}

type ParquetMetric struct {
	Labels  map[string]string
	Metrics map[string]*MetricInfo
}

func translateUnit(metric *pmetric.Metric, descriptor map[string]MetricDescriptor) string {
	unit := metric.Unit()
	if descriptor, exists := descriptor[metric.Name()]; exists {
		if unit == "" || descriptor.overwrite {
			return descriptor.unit
		}
	}
	switch unit {
	case "ms":
		unit = "Milliseconds"
	case "s":
		unit = "Seconds"
	case "us":
		unit = "Microseconds"
	case "By":
		unit = "Bytes"
	case "Bi":
		unit = "Bits"
	}
	return unit
}

//func addToParquetMetric(pmd *pdata.Metric, parquetMetrics map[interface{}]*ParquetMetric, logger *zap.Logger, descriptor map[string]MetricDescriptor) {
func addToParquetMetric(pmd *pmetric.Metric,
	parquetMetrics *[]*ParquetMetric, instrumentationLibName string,
	logger *zap.Logger, descriptor map[string]MetricDescriptor) {
	if pmd == nil {
		return
	}

	metricName := pmd.Name()
	dps := getDataPoints(pmd, instrumentationLibName, logger)
	if dps == nil || dps.Len() == 0 {
		return
	}

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		metric := &MetricInfo{
			Value: dp.Value,
			Unit:  translateUnit(pmd, descriptor),
		}

		*parquetMetrics = append(*parquetMetrics, &ParquetMetric{
			Labels:  dp.Labels,
			Metrics: map[string]*MetricInfo{(metricName): metric},
		})
	}
}

// translateOTelToGroupedMetric converts OT metrics to schema specified in parquet writer.
func (mt metricTranslator) translateOTelToParquetMetric(rm *pmetric.ResourceMetrics,
	parquetMetrics *[]*ParquetMetric, config *Config) {
	var instrumentationLibName string

	ilms := rm.ScopeMetrics()

	for j := 0; j < ilms.Len(); j++ {
		ilm := ilms.At(j)
		if ilm.Scope().Name() == "" {
			instrumentationLibName = _noInstrumentationLibraryName
		} else {
			instrumentationLibName = ilm.Scope().Name()
		}

		metrics := ilm.Metrics()
		for k := 0; k < metrics.Len(); k++ {
			metric := metrics.At(k)
			addToParquetMetric(&metric, parquetMetrics, instrumentationLibName, config.logger, mt.metricDescriptor)
		}
	}

}
