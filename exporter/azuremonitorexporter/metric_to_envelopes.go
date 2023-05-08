// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricPacker struct {
	logger *zap.Logger
}

type timedMetricDataPoint struct {
	dataPoint  *contracts.DataPoint
	timestamp  pcommon.Timestamp
	attributes pcommon.Map
}

type metricTimedData interface {
	getTimedDataPoints() []*timedMetricDataPoint
}

// MetricToEnvelopes packages metrics into a slice of Application Insight envelopes.
func (packer *metricPacker) MetricToEnvelopes(metric pmetric.Metric, resource pcommon.Resource, instrumentationScope pcommon.InstrumentationScope) []*contracts.Envelope {
	var envelopes []*contracts.Envelope

	mtd := packer.getMetricTimedData(metric)

	if mtd != nil {

		for _, timedDataPoint := range mtd.getTimedDataPoints() {

			envelope := contracts.NewEnvelope()
			envelope.Tags = make(map[string]string)
			envelope.Time = toTime(timedDataPoint.timestamp).Format(time.RFC3339Nano)

			metricData := contracts.NewMetricData()
			dataPoint := timedDataPoint.dataPoint
			metricData.Metrics = []*contracts.DataPoint{dataPoint}
			metricData.Properties = make(map[string]string)

			envelope.Name = metricData.EnvelopeName("")

			data := contracts.NewData()
			data.BaseData = metricData
			data.BaseType = metricData.BaseType()
			envelope.Data = data

			resourceAttributes := resource.Attributes()
			applyResourcesToDataProperties(metricData.Properties, resourceAttributes)
			applyInstrumentationScopeValueToDataProperties(metricData.Properties, instrumentationScope)
			applyCloudTagsToEnvelope(envelope, resourceAttributes)

			setAttributesAsProperties(timedDataPoint.attributes, metricData.Properties)

			packer.sanitize(func() []string { return metricData.Sanitize() })
			packer.sanitize(func() []string { return envelope.Sanitize() })
			packer.sanitize(func() []string { return contracts.SanitizeTags(envelope.Tags) })

			packer.logger.Debug("Metric is packed", zap.String("name", dataPoint.Name), zap.Any("value", dataPoint.Value))

			envelopes = append(envelopes, envelope)

		}
	}

	return envelopes
}

func (packer *metricPacker) sanitize(sanitizeFunc func() []string) {
	for _, warning := range sanitizeFunc() {
		packer.logger.Warn(warning)
	}
}

func newMetricPacker(logger *zap.Logger) *metricPacker {
	packer := &metricPacker{
		logger: logger,
	}
	return packer
}

func (packer metricPacker) getMetricTimedData(metric pmetric.Metric) metricTimedData {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		return newScalarMetric(metric.Name(), metric.Gauge().DataPoints())
	case pmetric.MetricTypeSum:
		return newScalarMetric(metric.Name(), metric.Sum().DataPoints())
	case pmetric.MetricTypeHistogram:
		return newHistogramMetric(metric.Name(), metric.Histogram().DataPoints())
	case pmetric.MetricTypeExponentialHistogram:
		return newExponentialHistogramMetric(metric.Name(), metric.ExponentialHistogram().DataPoints())
	case pmetric.MetricTypeSummary:
		return newSummaryMetric(metric.Name(), metric.Summary().DataPoints())
	}

	packer.logger.Debug("Unsupported metric type", zap.Any("Metric Type", metric.Type()))
	return nil
}

type scalarMetric struct {
	name           string
	dataPointSlice pmetric.NumberDataPointSlice
}

func newScalarMetric(name string, dataPointSlice pmetric.NumberDataPointSlice) *scalarMetric {
	return &scalarMetric{
		name:           name,
		dataPointSlice: dataPointSlice,
	}
}

func (m scalarMetric) getTimedDataPoints() []*timedMetricDataPoint {
	timedDataPoints := make([]*timedMetricDataPoint, m.dataPointSlice.Len())
	for i := 0; i < m.dataPointSlice.Len(); i++ {
		numberDataPoint := m.dataPointSlice.At(i)
		dataPoint := contracts.NewDataPoint()
		dataPoint.Name = m.name
		switch numberDataPoint.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			dataPoint.Value = numberDataPoint.DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			dataPoint.Value = float64(numberDataPoint.IntValue())
		case pmetric.NumberDataPointValueTypeEmpty:
			dataPoint.Value = 0
		}
		dataPoint.Count = 1
		dataPoint.Kind = contracts.Measurement
		timedDataPoints[i] = &timedMetricDataPoint{
			dataPoint:  dataPoint,
			timestamp:  numberDataPoint.Timestamp(),
			attributes: numberDataPoint.Attributes(),
		}
	}
	return timedDataPoints
}

type histogramMetric struct {
	name           string
	dataPointSlice pmetric.HistogramDataPointSlice
}

func newHistogramMetric(name string, dataPointSlice pmetric.HistogramDataPointSlice) *histogramMetric {
	return &histogramMetric{
		name:           name,
		dataPointSlice: dataPointSlice,
	}
}

func (m histogramMetric) getTimedDataPoints() []*timedMetricDataPoint {
	timedDataPoints := make([]*timedMetricDataPoint, m.dataPointSlice.Len())
	for i := 0; i < m.dataPointSlice.Len(); i++ {
		histogramDataPoint := m.dataPointSlice.At(i)
		dataPoint := contracts.NewDataPoint()
		dataPoint.Name = m.name
		dataPoint.Value = histogramDataPoint.Sum()
		dataPoint.Kind = contracts.Aggregation
		dataPoint.Min = histogramDataPoint.Min()
		dataPoint.Max = histogramDataPoint.Max()
		dataPoint.Count = int(histogramDataPoint.Count())

		timedDataPoints[i] = &timedMetricDataPoint{
			dataPoint:  dataPoint,
			timestamp:  histogramDataPoint.Timestamp(),
			attributes: histogramDataPoint.Attributes(),
		}

	}
	return timedDataPoints
}

type exponentialHistogramMetric struct {
	name           string
	dataPointSlice pmetric.ExponentialHistogramDataPointSlice
}

func newExponentialHistogramMetric(name string, dataPointSlice pmetric.ExponentialHistogramDataPointSlice) *exponentialHistogramMetric {
	return &exponentialHistogramMetric{
		name:           name,
		dataPointSlice: dataPointSlice,
	}
}

func (m exponentialHistogramMetric) getTimedDataPoints() []*timedMetricDataPoint {
	timedDataPoints := make([]*timedMetricDataPoint, m.dataPointSlice.Len())
	for i := 0; i < m.dataPointSlice.Len(); i++ {
		exponentialHistogramDataPoint := m.dataPointSlice.At(i)
		dataPoint := contracts.NewDataPoint()
		dataPoint.Name = m.name
		dataPoint.Value = exponentialHistogramDataPoint.Sum()
		dataPoint.Kind = contracts.Aggregation
		dataPoint.Min = exponentialHistogramDataPoint.Min()
		dataPoint.Max = exponentialHistogramDataPoint.Max()
		dataPoint.Count = int(exponentialHistogramDataPoint.Count())

		timedDataPoints[i] = &timedMetricDataPoint{
			dataPoint:  dataPoint,
			timestamp:  exponentialHistogramDataPoint.Timestamp(),
			attributes: exponentialHistogramDataPoint.Attributes(),
		}
	}
	return timedDataPoints
}

type summaryMetric struct {
	name           string
	dataPointSlice pmetric.SummaryDataPointSlice
}

func newSummaryMetric(name string, dataPointSlice pmetric.SummaryDataPointSlice) *summaryMetric {
	return &summaryMetric{
		name:           name,
		dataPointSlice: dataPointSlice,
	}
}

func (m summaryMetric) getTimedDataPoints() []*timedMetricDataPoint {
	timedDataPoints := make([]*timedMetricDataPoint, m.dataPointSlice.Len())
	for i := 0; i < m.dataPointSlice.Len(); i++ {
		summaryDataPoint := m.dataPointSlice.At(i)
		dataPoint := contracts.NewDataPoint()
		dataPoint.Name = m.name
		dataPoint.Value = summaryDataPoint.Sum()
		dataPoint.Kind = contracts.Aggregation
		dataPoint.Count = int(summaryDataPoint.Count())

		timedDataPoints[i] = &timedMetricDataPoint{
			dataPoint:  dataPoint,
			timestamp:  summaryDataPoint.Timestamp(),
			attributes: summaryDataPoint.Attributes(),
		}

	}
	return timedDataPoints
}
