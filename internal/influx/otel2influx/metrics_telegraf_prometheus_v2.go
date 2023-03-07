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

package otel2influx // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/otel2influx"

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/common"
)

type metricWriterTelegrafPrometheusV2 struct {
	logger common.Logger
}

func (c *metricWriterTelegrafPrometheusV2) writeMetric(ctx context.Context, resource pcommon.Resource, instrumentationLibrary pcommon.InstrumentationScope, metric pmetric.Metric, batch InfluxWriterBatch) error {
	// Ignore metric.Description() and metric.Unit() .
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		return c.writeGauge(ctx, resource, instrumentationLibrary, metric.Name(), metric.Gauge(), batch)
	case pmetric.MetricTypeSum:
		if metric.Sum().IsMonotonic() {
			return c.writeSum(ctx, resource, instrumentationLibrary, metric.Name(), metric.Sum(), batch)
		}
		return c.writeGaugeFromSum(ctx, resource, instrumentationLibrary, metric.Name(), metric.Sum(), batch)
	case pmetric.MetricTypeHistogram:
		return c.writeHistogram(ctx, resource, instrumentationLibrary, metric.Name(), metric.Histogram(), batch)
	case pmetric.MetricTypeSummary:
		return c.writeSummary(ctx, resource, instrumentationLibrary, metric.Name(), metric.Summary(), batch)
	default:
		return fmt.Errorf("unknown metric type %q", metric.Type())
	}
}

func (c *metricWriterTelegrafPrometheusV2) initMetricTagsAndTimestamp(resource pcommon.Resource, instrumentationLibrary pcommon.InstrumentationScope, dataPoint basicDataPoint) (tags map[string]string, fields map[string]interface{}, ts time.Time, err error) {
	ts = dataPoint.Timestamp().AsTime()
	if ts.IsZero() {
		err = errors.New("metric has no timestamp")
		return
	}

	tags = make(map[string]string)
	fields = make(map[string]interface{})
	if dataPoint.StartTimestamp() != 0 {
		fields[common.AttributeStartTimeUnixNano] = int64(dataPoint.StartTimestamp())
	}

	dataPoint.Attributes().Range(func(k string, v pcommon.Value) bool {
		if k == "" {
			c.logger.Debug("metric attribute key is empty")
		} else {
			var vv string
			vv, err = common.AttributeValueToInfluxTagValue(v)
			if err != nil {
				return false
			}
			tags[k] = vv
		}
		return true
	})
	if err != nil {
		err = fmt.Errorf("failed to convert attribute value to string: %w", err)
		return
	}

	tags = ResourceToTags(c.logger, resource, tags)
	tags = InstrumentationScopeToTags(instrumentationLibrary, tags)

	return
}

func (c *metricWriterTelegrafPrometheusV2) writeGauge(ctx context.Context, resource pcommon.Resource, instrumentationLibrary pcommon.InstrumentationScope, measurement string, gauge pmetric.Gauge, batch InfluxWriterBatch) error {
	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dataPoint := gauge.DataPoints().At(i)
		tags, fields, ts, err := c.initMetricTagsAndTimestamp(resource, instrumentationLibrary, dataPoint)
		if err != nil {
			return err
		}

		switch dataPoint.ValueType() {
		case pmetric.NumberDataPointValueTypeEmpty:
			continue
		case pmetric.NumberDataPointValueTypeDouble:
			fields[measurement] = dataPoint.DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			fields[measurement] = dataPoint.IntValue()
		default:
			return fmt.Errorf("unsupported gauge data point type %d", dataPoint.ValueType())
		}

		if err = batch.WritePoint(ctx, common.MeasurementPrometheus, tags, fields, ts, common.InfluxMetricValueTypeGauge); err != nil {
			return fmt.Errorf("failed to write point for gauge: %w", err)
		}
	}

	return nil
}

func (c *metricWriterTelegrafPrometheusV2) writeGaugeFromSum(ctx context.Context, resource pcommon.Resource, instrumentationLibrary pcommon.InstrumentationScope, measurement string, sum pmetric.Sum, batch InfluxWriterBatch) error {
	if sum.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		return fmt.Errorf("unsupported sum (as gauge) aggregation temporality %q", sum.AggregationTemporality())
	}

	for i := 0; i < sum.DataPoints().Len(); i++ {
		dataPoint := sum.DataPoints().At(i)
		tags, fields, ts, err := c.initMetricTagsAndTimestamp(resource, instrumentationLibrary, dataPoint)
		if err != nil {
			return err
		}

		switch dataPoint.ValueType() {
		case pmetric.NumberDataPointValueTypeEmpty:
			continue
		case pmetric.NumberDataPointValueTypeDouble:
			fields[measurement] = dataPoint.DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			fields[measurement] = dataPoint.IntValue()
		default:
			return fmt.Errorf("unsupported sum (as gauge) data point type %d", dataPoint.ValueType())
		}

		if err = batch.WritePoint(ctx, common.MeasurementPrometheus, tags, fields, ts, common.InfluxMetricValueTypeGauge); err != nil {
			return fmt.Errorf("failed to write point for sum (as gauge): %w", err)
		}
	}

	return nil
}

func (c *metricWriterTelegrafPrometheusV2) writeSum(ctx context.Context, resource pcommon.Resource, instrumentationLibrary pcommon.InstrumentationScope, measurement string, sum pmetric.Sum, batch InfluxWriterBatch) error {
	if sum.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		return fmt.Errorf("unsupported sum aggregation temporality %q", sum.AggregationTemporality())
	}

	for i := 0; i < sum.DataPoints().Len(); i++ {
		dataPoint := sum.DataPoints().At(i)
		tags, fields, ts, err := c.initMetricTagsAndTimestamp(resource, instrumentationLibrary, dataPoint)
		if err != nil {
			return err
		}

		switch dataPoint.ValueType() {
		case pmetric.NumberDataPointValueTypeEmpty:
			continue
		case pmetric.NumberDataPointValueTypeDouble:
			fields[measurement] = dataPoint.DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			fields[measurement] = dataPoint.IntValue()
		default:
			return fmt.Errorf("unsupported sum data point type %d", dataPoint.ValueType())
		}

		if err = batch.WritePoint(ctx, common.MeasurementPrometheus, tags, fields, ts, common.InfluxMetricValueTypeSum); err != nil {
			return fmt.Errorf("failed to write point for sum: %w", err)
		}
	}

	return nil
}

func (c *metricWriterTelegrafPrometheusV2) writeHistogram(ctx context.Context, resource pcommon.Resource, instrumentationLibrary pcommon.InstrumentationScope, measurement string, histogram pmetric.Histogram, batch InfluxWriterBatch) error {
	if histogram.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		return fmt.Errorf("unsupported histogram aggregation temporality %q", histogram.AggregationTemporality())
	}

	for i := 0; i < histogram.DataPoints().Len(); i++ {
		dataPoint := histogram.DataPoints().At(i)
		tags, fields, ts, err := c.initMetricTagsAndTimestamp(resource, instrumentationLibrary, dataPoint)
		if err != nil {
			return err
		}

		{
			f := make(map[string]interface{}, len(fields)+2)
			for k, v := range fields {
				f[k] = v
			}

			f[measurement+common.MetricHistogramCountSuffix] = float64(dataPoint.Count())
			f[measurement+common.MetricHistogramSumSuffix] = dataPoint.Sum()

			if err = batch.WritePoint(ctx, common.MeasurementPrometheus, tags, f, ts, common.InfluxMetricValueTypeHistogram); err != nil {
				return fmt.Errorf("failed to write point for histogram: %w", err)
			}
		}

		bucketCounts, explicitBounds := dataPoint.BucketCounts(), dataPoint.ExplicitBounds()
		if bucketCounts.Len() > 0 &&
			bucketCounts.Len() != explicitBounds.Len() &&
			bucketCounts.Len() != explicitBounds.Len()+1 {
			// The infinity bucket is not used in this schema,
			// so accept input if that particular bucket is missing.
			return fmt.Errorf("invalid metric histogram bucket counts qty %d vs explicit bounds qty %d", bucketCounts.Len(), explicitBounds.Len())
		}

		for i := 0; i < explicitBounds.Len(); i++ {
			t := make(map[string]string, len(tags)+1)
			for k, v := range tags {
				t[k] = v
			}
			f := make(map[string]interface{}, len(fields)+1)
			for k, v := range fields {
				f[k] = v
			}

			boundTagValue := strconv.FormatFloat(explicitBounds.At(i), 'f', -1, 64)
			t[common.MetricHistogramBoundKeyV2] = boundTagValue
			f[measurement+common.MetricHistogramBucketSuffix] = float64(bucketCounts.At(i))

			if err = batch.WritePoint(ctx, common.MeasurementPrometheus, t, f, ts, common.InfluxMetricValueTypeHistogram); err != nil {
				return fmt.Errorf("failed to write point for histogram: %w", err)
			}
		}
	}

	return nil
}

func (c *metricWriterTelegrafPrometheusV2) writeSummary(ctx context.Context, resource pcommon.Resource, instrumentationLibrary pcommon.InstrumentationScope, measurement string, summary pmetric.Summary, batch InfluxWriterBatch) error {
	for i := 0; i < summary.DataPoints().Len(); i++ {
		dataPoint := summary.DataPoints().At(i)
		tags, fields, ts, err := c.initMetricTagsAndTimestamp(resource, instrumentationLibrary, dataPoint)
		if err != nil {
			return err
		}

		{
			f := make(map[string]interface{}, len(fields)+2)
			for k, v := range fields {
				f[k] = v
			}

			f[measurement+common.MetricSummaryCountSuffix] = float64(dataPoint.Count())
			f[measurement+common.MetricSummarySumSuffix] = dataPoint.Sum()

			if err = batch.WritePoint(ctx, common.MeasurementPrometheus, tags, f, ts, common.InfluxMetricValueTypeSummary); err != nil {
				return fmt.Errorf("failed to write point for summary: %w", err)
			}
		}

		for j := 0; j < dataPoint.QuantileValues().Len(); j++ {
			valueAtQuantile := dataPoint.QuantileValues().At(j)
			t := make(map[string]string, len(tags)+1)
			for k, v := range tags {
				t[k] = v
			}
			f := make(map[string]interface{}, len(fields)+1)
			for k, v := range fields {
				f[k] = v
			}

			quantileTagValue := strconv.FormatFloat(valueAtQuantile.Quantile(), 'f', -1, 64)
			t[common.MetricSummaryQuantileKeyV2] = quantileTagValue
			f[measurement] = valueAtQuantile.Value()

			if err = batch.WritePoint(ctx, common.MeasurementPrometheus, t, f, ts, common.InfluxMetricValueTypeSummary); err != nil {
				return fmt.Errorf("failed to write point for summary: %w", err)
			}
		}
	}

	return nil
}
