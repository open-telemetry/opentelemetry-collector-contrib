// Copyright OpenTelemetry Authors
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

package signalfx // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx"

import (
	"math"
	"strconv"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Some fields on SignalFx protobuf are pointers, in order to reduce
// allocations create the most used ones.
var (
	// SignalFx metric types used in the conversions.
	sfxMetricTypeGauge             = sfxpb.MetricType_GAUGE
	sfxMetricTypeCumulativeCounter = sfxpb.MetricType_CUMULATIVE_COUNTER
	sfxMetricTypeCounter           = sfxpb.MetricType_COUNTER
)

var (
	// Some standard dimension keys.
	// upper bound dimension key for histogram buckets.
	upperBoundDimensionKey = "upper_bound"

	// infinity bound dimension value is used on all histograms.
	infinityBoundSFxDimValue = float64ToDimValue(math.Inf(1))
)

// FromMetrics converts pmetric.Metrics to SignalFx proto data points.
func FromMetrics(md pmetric.Metrics) ([]*sfxpb.DataPoint, error) {
	var sfxDataPoints []*sfxpb.DataPoint

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		extraDimensions := attributesToDimensions(rm.Resource().Attributes(), nil)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				sfxDataPoints = append(sfxDataPoints, FromMetric(ilm.Metrics().At(k), extraDimensions)...)
			}
		}
	}

	return sfxDataPoints, nil
}

// FromMetric converts pmetric.Metric to SignalFx proto data points.
// TODO: Remove this and change signalfxexporter to us FromMetrics.
func FromMetric(m pmetric.Metric, extraDimensions []*sfxpb.Dimension) []*sfxpb.DataPoint {
	var dps []*sfxpb.DataPoint

	basePoint := &sfxpb.DataPoint{
		Metric:     m.Name(),
		MetricType: fromMetricTypeToMetricType(m),
	}

	switch m.DataType() {
	case pmetric.MetricDataTypeGauge:
		dps = convertNumberDataPoints(m.Gauge().DataPoints(), basePoint, extraDimensions)
	case pmetric.MetricDataTypeSum:
		dps = convertNumberDataPoints(m.Sum().DataPoints(), basePoint, extraDimensions)
	case pmetric.MetricDataTypeHistogram:
		dps = convertHistogram(m.Histogram().DataPoints(), basePoint, extraDimensions)
	case pmetric.MetricDataTypeSummary:
		dps = convertSummaryDataPoints(m.Summary().DataPoints(), m.Name(), extraDimensions)
	}

	return dps
}

func fromMetricTypeToMetricType(metric pmetric.Metric) *sfxpb.MetricType {
	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		return &sfxMetricTypeGauge

	case pmetric.MetricDataTypeSum:
		if !metric.Sum().IsMonotonic() {
			return &sfxMetricTypeGauge
		}
		if metric.Sum().AggregationTemporality() == pmetric.MetricAggregationTemporalityDelta {
			return &sfxMetricTypeCounter
		}
		return &sfxMetricTypeCumulativeCounter

	case pmetric.MetricDataTypeHistogram:
		if metric.Histogram().AggregationTemporality() == pmetric.MetricAggregationTemporalityDelta {
			return &sfxMetricTypeCounter
		}
		return &sfxMetricTypeCumulativeCounter
	}

	return nil
}

func convertNumberDataPoints(in pmetric.NumberDataPointSlice, basePoint *sfxpb.DataPoint, extraDims []*sfxpb.Dimension) []*sfxpb.DataPoint {
	out := make([]*sfxpb.DataPoint, 0, in.Len())

	for i := 0; i < in.Len(); i++ {
		inDp := in.At(i)

		dp := *basePoint
		dp.Timestamp = fromTimestamp(inDp.Timestamp())
		dp.Dimensions = attributesToDimensions(inDp.Attributes(), extraDims)

		switch inDp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			val := inDp.IntVal()
			dp.Value.IntValue = &val
		case pmetric.NumberDataPointValueTypeDouble:
			val := inDp.DoubleVal()
			dp.Value.DoubleValue = &val
		}

		out = append(out, &dp)
	}
	return out
}

func convertHistogram(histDPs pmetric.HistogramDataPointSlice, basePoint *sfxpb.DataPoint, extraDims []*sfxpb.Dimension) []*sfxpb.DataPoint {
	var out []*sfxpb.DataPoint

	for i := 0; i < histDPs.Len(); i++ {
		histDP := histDPs.At(i)
		ts := fromTimestamp(histDP.Timestamp())

		countDP := *basePoint
		countDP.Metric = basePoint.Metric + "_count"
		countDP.Timestamp = ts
		countDP.Dimensions = attributesToDimensions(histDP.Attributes(), extraDims)
		count := int64(histDP.Count())
		countDP.Value.IntValue = &count

		sumDP := *basePoint
		sumDP.Timestamp = ts
		sumDP.Dimensions = attributesToDimensions(histDP.Attributes(), extraDims)
		sum := histDP.Sum()
		sumDP.Value.DoubleValue = &sum

		out = append(out, &countDP, &sumDP)

		bounds := histDP.ExplicitBounds()
		counts := histDP.BucketCounts()

		// Spec says counts is optional but if present it must have one more
		// element than the bounds array.
		if len(counts) > 0 && len(counts) != len(bounds)+1 {
			continue
		}

		for j, c := range counts {
			bound := infinityBoundSFxDimValue
			if j < len(bounds) {
				bound = float64ToDimValue(bounds[j])
			}

			dp := *basePoint
			dp.Metric = basePoint.Metric + "_bucket"
			dp.Timestamp = ts
			dp.Dimensions = attributesToDimensions(histDP.Attributes(), extraDims)
			dp.Dimensions = append(dp.Dimensions, &sfxpb.Dimension{
				Key:   upperBoundDimensionKey,
				Value: bound,
			})
			cInt := int64(c)
			dp.Value.IntValue = &cInt

			out = append(out, &dp)
		}
	}

	return out
}

func convertSummaryDataPoints(
	in pmetric.SummaryDataPointSlice,
	name string,
	extraDims []*sfxpb.Dimension,
) []*sfxpb.DataPoint {
	out := make([]*sfxpb.DataPoint, 0, in.Len())

	for i := 0; i < in.Len(); i++ {
		inDp := in.At(i)

		dims := attributesToDimensions(inDp.Attributes(), extraDims)
		ts := fromTimestamp(inDp.Timestamp())

		countPt := sfxpb.DataPoint{
			Metric:     name + "_count",
			Timestamp:  ts,
			Dimensions: dims,
			MetricType: &sfxMetricTypeCumulativeCounter,
		}
		c := int64(inDp.Count())
		countPt.Value.IntValue = &c
		out = append(out, &countPt)

		sumPt := sfxpb.DataPoint{
			Metric:     name,
			Timestamp:  ts,
			Dimensions: dims,
			MetricType: &sfxMetricTypeCumulativeCounter,
		}
		sum := inDp.Sum()
		sumPt.Value.DoubleValue = &sum
		out = append(out, &sumPt)

		qvs := inDp.QuantileValues()
		for j := 0; j < qvs.Len(); j++ {
			qPt := sfxpb.DataPoint{
				Metric:     name + "_quantile",
				Timestamp:  ts,
				MetricType: &sfxMetricTypeGauge,
			}
			qv := qvs.At(j)
			qdim := sfxpb.Dimension{
				Key:   "quantile",
				Value: strconv.FormatFloat(qv.Quantile(), 'f', -1, 64),
			}
			qPt.Dimensions = append(dims, &qdim)
			v := qv.Value()
			qPt.Value.DoubleValue = &v
			out = append(out, &qPt)
		}
	}
	return out
}

func attributesToDimensions(attributes pcommon.Map, extraDims []*sfxpb.Dimension) []*sfxpb.Dimension {
	dimensions := make([]*sfxpb.Dimension, len(extraDims), attributes.Len()+len(extraDims))
	copy(dimensions, extraDims)
	if attributes.Len() == 0 {
		return dimensions
	}
	dimensionsValue := make([]sfxpb.Dimension, attributes.Len())
	pos := 0
	attributes.Range(func(k string, v pcommon.Value) bool {
		dimensionsValue[pos].Key = k
		dimensionsValue[pos].Value = v.AsString()
		dimensions = append(dimensions, &dimensionsValue[pos])
		pos++
		return true
	})
	return dimensions
}

// Is equivalent to strconv.FormatFloat(f, 'g', -1, 64), but hardcodes a few common cases for increased efficiency.
func float64ToDimValue(f float64) string {
	// Parameters below are the same used by Prometheus
	// see https://github.com/prometheus/common/blob/b5fe7d854c42dc7842e48d1ca58f60feae09d77b/expfmt/text_create.go#L450
	// SignalFx agent uses a different pattern
	// https://github.com/signalfx/signalfx-agent/blob/5779a3de0c9861fa07316fd11b3c4ff38c0d78f0/internal/monitors/prometheusexporter/conversion.go#L77
	// The important issue here is consistency with the exporter, opting for the
	// more common one used by Prometheus.
	switch {
	case f == 0:
		return "0"
	case f == 1:
		return "1"
	case math.IsInf(f, +1):
		return "+Inf"
	default:
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
}
