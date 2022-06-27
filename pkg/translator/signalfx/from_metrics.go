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

	// infinity bound dimension value is used on all histograms.
	infinityBoundSFxDimValue = float64ToDimValue(math.Inf(1))
)

const (
	// upper bound dimension key for histogram buckets.
	bucketDimensionKey = "upper_bound"

	// prometheus compatible dimension key for histogram buckets.
	prometheusBucketDimensionKey = "le"

	// quantile dimension key for summary quantiles.
	quantileDimensionKey = "quantile"
)

// FromTranslator converts from pdata to SignalFx proto data model.
type FromTranslator struct {
	// PrometheusCompatible controls if conversion should follow prometheus compatibility for histograms and summaries.
	// If false it emits old signalfx smart agent format.
	PrometheusCompatible bool
}

// FromMetrics converts pmetric.Metrics to SignalFx proto data points.
func (ft *FromTranslator) FromMetrics(md pmetric.Metrics) ([]*sfxpb.DataPoint, error) {
	var sfxDataPoints []*sfxpb.DataPoint

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		extraDimensions := attributesToDimensions(rm.Resource().Attributes(), nil)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				sfxDataPoints = append(sfxDataPoints, ft.FromMetric(ilm.Metrics().At(k), extraDimensions)...)
			}
		}
	}

	return sfxDataPoints, nil
}

// FromMetric converts pmetric.Metric to SignalFx proto data points.
// TODO: Remove this and change signalfxexporter to us FromMetrics.
func (ft *FromTranslator) FromMetric(m pmetric.Metric, extraDimensions []*sfxpb.Dimension) []*sfxpb.DataPoint {
	var dps []*sfxpb.DataPoint

	mt := fromMetricTypeToMetricType(m)

	switch m.DataType() {
	case pmetric.MetricDataTypeGauge:
		dps = convertNumberDataPoints(m.Gauge().DataPoints(), m.Name(), mt, extraDimensions)
	case pmetric.MetricDataTypeSum:
		dps = convertNumberDataPoints(m.Sum().DataPoints(), m.Name(), mt, extraDimensions)
	case pmetric.MetricDataTypeHistogram:
		dps = convertHistogram(m.Histogram().DataPoints(), m.Name(), mt, extraDimensions, ft.PrometheusCompatible)
	case pmetric.MetricDataTypeSummary:
		dps = convertSummaryDataPoints(m.Summary().DataPoints(), m.Name(), extraDimensions, ft.PrometheusCompatible)
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

func convertNumberDataPoints(in pmetric.NumberDataPointSlice, name string, mt *sfxpb.MetricType, extraDims []*sfxpb.Dimension) []*sfxpb.DataPoint {
	dps := newDpsBuilder(in.Len())

	for i := 0; i < in.Len(); i++ {
		inDp := in.At(i)

		dp := dps.appendPoint(name, mt, fromTimestamp(inDp.Timestamp()), attributesToDimensions(inDp.Attributes(), extraDims))
		switch inDp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			val := inDp.IntVal()
			dp.Value.IntValue = &val
		case pmetric.NumberDataPointValueTypeDouble:
			val := inDp.DoubleVal()
			dp.Value.DoubleValue = &val
		}
	}
	return dps.out
}

func convertHistogram(in pmetric.HistogramDataPointSlice, name string, mt *sfxpb.MetricType, extraDims []*sfxpb.Dimension, promCompatible bool) []*sfxpb.DataPoint {
	var numDPs int
	for i := 0; i < in.Len(); i++ {
		numDPs += 2 + in.At(i).BucketCounts().Len()
	}
	dps := newDpsBuilder(numDPs)

	for i := 0; i < in.Len(); i++ {
		histDP := in.At(i)
		ts := fromTimestamp(histDP.Timestamp())
		dims := attributesToDimensions(histDP.Attributes(), extraDims)

		countDP := dps.appendPoint(name+"_count", mt, ts, dims)
		count := int64(histDP.Count())
		countDP.Value.IntValue = &count

		sumName := name
		if promCompatible {
			sumName = name + "_sum"
		}
		sumDP := dps.appendPoint(sumName, mt, ts, dims)
		sum := histDP.Sum()
		sumDP.Value.DoubleValue = &sum

		bounds := histDP.ExplicitBounds()
		counts := histDP.BucketCounts()

		// Spec says counts is optional but if present it must have one more
		// element than the bounds array.
		if counts.Len() > 0 && counts.Len() != bounds.Len()+1 {
			continue
		}

		bucketMetricName := name + "_bucket"
		bdKey := bucketDimensionKey
		if promCompatible {
			bdKey = prometheusBucketDimensionKey
		}
		var val uint64
		for j := 0; j < counts.Len(); j++ {
			val += counts.At(j)
			bound := infinityBoundSFxDimValue
			if j < bounds.Len() {
				bound = float64ToDimValue(bounds.At(j))
			}
			cloneDim := make([]*sfxpb.Dimension, len(dims)+1)
			copy(cloneDim, dims)
			cloneDim[len(dims)] = &sfxpb.Dimension{
				Key:   bdKey,
				Value: bound,
			}
			dp := dps.appendPoint(bucketMetricName, mt, ts, cloneDim)
			cInt := int64(val)
			dp.Value.IntValue = &cInt
		}
	}

	return dps.out
}

func convertSummaryDataPoints(in pmetric.SummaryDataPointSlice, name string, extraDims []*sfxpb.Dimension, promCompatible bool) []*sfxpb.DataPoint {
	var numDPs int
	for i := 0; i < in.Len(); i++ {
		numDPs += 2 + in.At(i).QuantileValues().Len()
	}
	dps := newDpsBuilder(numDPs)

	for i := 0; i < in.Len(); i++ {
		inDp := in.At(i)

		dims := attributesToDimensions(inDp.Attributes(), extraDims)
		ts := fromTimestamp(inDp.Timestamp())

		countDP := dps.appendPoint(name+"_count", &sfxMetricTypeCumulativeCounter, ts, dims)
		c := int64(inDp.Count())
		countDP.Value.IntValue = &c

		sumName := name
		if promCompatible {
			sumName = name + "_sum"
		}
		sumDP := dps.appendPoint(sumName, &sfxMetricTypeCumulativeCounter, ts, dims)
		sum := inDp.Sum()
		sumDP.Value.DoubleValue = &sum

		qvs := inDp.QuantileValues()
		for j := 0; j < qvs.Len(); j++ {
			qv := qvs.At(j)
			cloneDim := make([]*sfxpb.Dimension, len(dims)+1)
			copy(cloneDim, dims)
			cloneDim[len(dims)] = &sfxpb.Dimension{
				Key:   quantileDimensionKey,
				Value: strconv.FormatFloat(qv.Quantile(), 'f', -1, 64),
			}
			qPt := dps.appendPoint(name+"_quantile", &sfxMetricTypeGauge, ts, cloneDim)
			v := qv.Value()
			qPt.Value.DoubleValue = &v
		}
	}
	return dps.out
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

type dpsBuilder struct {
	baseOut []sfxpb.DataPoint
	out     []*sfxpb.DataPoint
	pos     int
}

func newDpsBuilder(cap int) dpsBuilder {
	return dpsBuilder{
		baseOut: make([]sfxpb.DataPoint, cap),
		out:     make([]*sfxpb.DataPoint, 0, cap),
	}
}

func (dp *dpsBuilder) appendPoint(name string, mt *sfxpb.MetricType, ts int64, dims []*sfxpb.Dimension) *sfxpb.DataPoint {
	base := dp.baseOut[dp.pos]
	dp.pos++
	dp.out = append(dp.out, &base)
	base.Metric = name
	base.Timestamp = ts
	base.MetricType = mt
	base.Dimensions = dims
	return &base
}

// Is equivalent to strconv.FormatFloat(f, 'g', -1, 64), but hard-codes a few common cases for increased efficiency.
func float64ToDimValue(f float64) string {
	// Parameters below are the same used by Prometheus
	// see https://github.com/prometheus/common/blob/b5fe7d854c42dc7842e48d1ca58f60feae09d77b/expfmt/text_create.go#L450
	// SignalFx agent uses a different pattern
	// https://github.com/signalfx/signalfx-agent/blob/5779a3de0c9861fa07316fd11b3c4ff38c0d78f0/internal/monitors/prometheusexporter/conversion.go#L77
	// The important issue here is consistency with the exporter, opting for the more common one used by Prometheus.
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
