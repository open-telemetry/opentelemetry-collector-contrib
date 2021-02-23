// Copyright 2020, OpenTelemetry Authors
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

package translation

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/consumer/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation/dpfilters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// Some fields on SignalFx protobuf are pointers, in order to reduce
// allocations create the most used ones.
var (
	// SignalFx metric types used in the conversions.
	sfxMetricTypeGauge             = sfxpb.MetricType_GAUGE
	sfxMetricTypeCumulativeCounter = sfxpb.MetricType_CUMULATIVE_COUNTER
	sfxMetricTypeCounter           = sfxpb.MetricType_COUNTER

	// Some standard dimension keys.
	// upper bound dimension key for histogram buckets.
	upperBoundDimensionKey = "upper_bound"

	// infinity bound dimension value is used on all histograms.
	infinityBoundSFxDimValue = float64ToDimValue(math.Inf(1))
)

// MetricsConverter converts MetricsData to sfxpb DataPoints. It holds an optional
// MetricTranslator to translate SFx metrics using translation rules.
type MetricsConverter struct {
	logger                  *zap.Logger
	metricTranslator        *MetricTranslator
	filterSet               *dpfilters.FilterSet
	nonAlphanumericDimChars string
}

// NewMetricsConverter creates a MetricsConverter from the passed in logger and
// MetricTranslator. Pass in a nil MetricTranslator to not use translation
// rules.
func NewMetricsConverter(
	logger *zap.Logger,
	t *MetricTranslator,
	excludes []dpfilters.MetricFilter,
	includes []dpfilters.MetricFilter,
	nonAlphanumericDimChars string) (*MetricsConverter, error) {
	fs, err := dpfilters.NewFilterSet(excludes, includes)
	if err != nil {
		return nil, err
	}
	return &MetricsConverter{logger: logger, metricTranslator: t, filterSet: fs, nonAlphanumericDimChars: nonAlphanumericDimChars}, nil
}

// MetricDataToSignalFxV2 converts the passed in MetricsData to SFx datapoints,
// returning those datapoints and the number of time series that had to be
// dropped because of errors or warnings.
func (c *MetricsConverter) MetricDataToSignalFxV2(rm pdata.ResourceMetrics) []*sfxpb.DataPoint {
	var sfxDatapoints []*sfxpb.DataPoint

	extraDimensions := resourceToDimensions(rm.Resource())

	for j := 0; j < rm.InstrumentationLibraryMetrics().Len(); j++ {
		ilm := rm.InstrumentationLibraryMetrics().At(j)
		for k := 0; k < ilm.Metrics().Len(); k++ {
			dps := c.metricToSfxDataPoints(ilm.Metrics().At(k), extraDimensions)
			sfxDatapoints = append(sfxDatapoints, dps...)
		}
	}
	c.sanitizeDataPointDimensions(sfxDatapoints)
	return sfxDatapoints
}

func (c *MetricsConverter) metricToSfxDataPoints(metric pdata.Metric, extraDimensions []*sfxpb.Dimension) []*sfxpb.DataPoint {
	// TODO: Figure out some efficient way to know how many datapoints there
	// will be in the given metric.
	var dps []*sfxpb.DataPoint

	basePoint := makeBaseDataPoint(metric)

	switch metric.DataType() {
	case pdata.MetricDataTypeNone:
		return nil
	case pdata.MetricDataTypeIntGauge:
		dps = convertIntDatapoints(metric.IntGauge().DataPoints(), basePoint, extraDimensions)
	case pdata.MetricDataTypeIntSum:
		dps = convertIntDatapoints(metric.IntSum().DataPoints(), basePoint, extraDimensions)
	case pdata.MetricDataTypeDoubleGauge:
		dps = convertDoubleDatapoints(metric.DoubleGauge().DataPoints(), basePoint, extraDimensions)
	case pdata.MetricDataTypeDoubleSum:
		dps = convertDoubleDatapoints(metric.DoubleSum().DataPoints(), basePoint, extraDimensions)
	case pdata.MetricDataTypeIntHistogram:
		dps = convertIntHistogram(metric.IntHistogram().DataPoints(), basePoint, extraDimensions)
	case pdata.MetricDataTypeDoubleHistogram:
		dps = convertDoubleHistogram(metric.DoubleHistogram().DataPoints(), basePoint, extraDimensions)
	}

	if c.metricTranslator != nil {
		dps = c.metricTranslator.TranslateDataPoints(c.logger, dps)
	}

	// TODO:
	// 1) Add hard coded list of metrics to be excluded to omit non-default metrics
	// 2) Add an include_metrics options that will serve as an override to the exclude
	// list. This will help to include metrics that are excluded by default.
	resultSliceLen := 0
	for i, dp := range dps {
		if !c.filterSet.Matches(dp) {
			if resultSliceLen < i {
				dps[resultSliceLen] = dp
			}
			resultSliceLen++
		}
	}
	dps = dps[:resultSliceLen]

	return dps
}

func labelsToDimensions(labels pdata.StringMap, extraDims []*sfxpb.Dimension) []*sfxpb.Dimension {
	dimensions := make([]*sfxpb.Dimension, len(extraDims), labels.Len()+len(extraDims))
	copy(dimensions, extraDims)
	if labels.Len() == 0 {
		return dimensions
	}
	dimensionsValue := make([]sfxpb.Dimension, labels.Len())
	pos := 0
	labels.ForEach(func(k string, v string) {
		dimensionsValue[pos].Key = k
		dimensionsValue[pos].Value = v
		dimensions = append(dimensions, &dimensionsValue[pos])
		pos++
	})
	return dimensions
}

func convertIntDatapoints(in pdata.IntDataPointSlice, basePoint *sfxpb.DataPoint, extraDims []*sfxpb.Dimension) []*sfxpb.DataPoint {
	out := make([]*sfxpb.DataPoint, 0, in.Len())

	for i := 0; i < in.Len(); i++ {
		inDp := in.At(i)

		dp := *basePoint
		dp.Timestamp = timestampToSignalFx(inDp.Timestamp())
		dp.Dimensions = labelsToDimensions(inDp.LabelsMap(), extraDims)

		val := inDp.Value()
		dp.Value.IntValue = &val

		out = append(out, &dp)
	}
	return out
}

func convertDoubleDatapoints(in pdata.DoubleDataPointSlice, basePoint *sfxpb.DataPoint, extraDims []*sfxpb.Dimension) []*sfxpb.DataPoint {
	out := make([]*sfxpb.DataPoint, 0, in.Len())

	for i := 0; i < in.Len(); i++ {
		inDp := in.At(i)

		dp := *basePoint
		dp.Timestamp = timestampToSignalFx(inDp.Timestamp())
		dp.Dimensions = labelsToDimensions(inDp.LabelsMap(), extraDims)

		val := inDp.Value()
		dp.Value.DoubleValue = &val

		out = append(out, &dp)
	}
	return out
}

func makeBaseDataPoint(m pdata.Metric) *sfxpb.DataPoint {
	return &sfxpb.DataPoint{
		Metric:     m.Name(),
		MetricType: fromMetricDataTypeToMetricType(m),
	}
}

func fromMetricDataTypeToMetricType(metric pdata.Metric) *sfxpb.MetricType {
	switch metric.DataType() {

	case pdata.MetricDataTypeIntGauge:
		return &sfxMetricTypeGauge

	case pdata.MetricDataTypeDoubleGauge:
		return &sfxMetricTypeGauge

	case pdata.MetricDataTypeIntSum:
		if !metric.IntSum().IsMonotonic() {
			return &sfxMetricTypeGauge
		}
		if metric.IntSum().AggregationTemporality() == pdata.AggregationTemporalityDelta {
			return &sfxMetricTypeCounter
		}
		return &sfxMetricTypeCumulativeCounter

	case pdata.MetricDataTypeDoubleSum:
		if !metric.DoubleSum().IsMonotonic() {
			return &sfxMetricTypeGauge
		}
		if metric.DoubleSum().AggregationTemporality() == pdata.AggregationTemporalityDelta {
			return &sfxMetricTypeCounter
		}
		return &sfxMetricTypeCumulativeCounter

	case pdata.MetricDataTypeIntHistogram:
		if metric.IntHistogram().AggregationTemporality() == pdata.AggregationTemporalityDelta {
			return &sfxMetricTypeCounter
		}
		return &sfxMetricTypeCumulativeCounter

	case pdata.MetricDataTypeDoubleHistogram:
		if metric.DoubleHistogram().AggregationTemporality() == pdata.AggregationTemporalityDelta {
			return &sfxMetricTypeCounter
		}
		return &sfxMetricTypeCumulativeCounter
	}

	return nil
}

func convertIntHistogram(histDPs pdata.IntHistogramDataPointSlice, basePoint *sfxpb.DataPoint, extraDims []*sfxpb.Dimension) []*sfxpb.DataPoint {
	var out []*sfxpb.DataPoint

	for i := 0; i < histDPs.Len(); i++ {
		histDP := histDPs.At(i)
		ts := timestampToSignalFx(histDP.Timestamp())

		countDP := *basePoint
		countDP.Metric = basePoint.Metric + "_count"
		countDP.Timestamp = ts
		countDP.Dimensions = labelsToDimensions(histDP.LabelsMap(), extraDims)
		count := int64(histDP.Count())
		countDP.Value.IntValue = &count

		sumDP := *basePoint
		sumDP.Timestamp = ts
		sumDP.Dimensions = labelsToDimensions(histDP.LabelsMap(), extraDims)
		sum := histDP.Sum()
		sumDP.Value.IntValue = &sum

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
			dp.Dimensions = labelsToDimensions(histDP.LabelsMap(), extraDims)
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

func convertDoubleHistogram(histDPs pdata.DoubleHistogramDataPointSlice, basePoint *sfxpb.DataPoint, extraDims []*sfxpb.Dimension) []*sfxpb.DataPoint {
	var out []*sfxpb.DataPoint

	for i := 0; i < histDPs.Len(); i++ {
		histDP := histDPs.At(i)
		ts := timestampToSignalFx(histDP.Timestamp())

		countDP := *basePoint
		countDP.Metric = basePoint.Metric + "_count"
		countDP.Timestamp = ts
		countDP.Dimensions = labelsToDimensions(histDP.LabelsMap(), extraDims)
		count := int64(histDP.Count())
		countDP.Value.IntValue = &count

		sumDP := *basePoint
		sumDP.Timestamp = ts
		sumDP.Dimensions = labelsToDimensions(histDP.LabelsMap(), extraDims)
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
			dp.Dimensions = labelsToDimensions(histDP.LabelsMap(), extraDims)
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

// sanitizeDataPointLabels replaces all characters unsupported by SignalFx backend
// in metric label keys and with "_"
func (c *MetricsConverter) sanitizeDataPointDimensions(dps []*sfxpb.DataPoint) {
	for _, dp := range dps {
		for _, d := range dp.Dimensions {
			d.Key = filterKeyChars(d.Key, c.nonAlphanumericDimChars)
		}
	}
}

func filterKeyChars(str string, nonAlphanumericDimChars string) string {
	filterMap := func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(nonAlphanumericDimChars, r) {
			return r
		}
		return '_'
	}

	return strings.Map(filterMap, str)
}

func float64ToDimValue(f float64) string {
	// Parameters below are the same used by Prometheus
	// see https://github.com/prometheus/common/blob/b5fe7d854c42dc7842e48d1ca58f60feae09d77b/expfmt/text_create.go#L450
	// SignalFx agent uses a different pattern
	// https://github.com/signalfx/signalfx-agent/blob/5779a3de0c9861fa07316fd11b3c4ff38c0d78f0/internal/monitors/prometheusexporter/conversion.go#L77
	// The important issue here is consistency with the exporter, opting for the
	// more common one used by Prometheus.
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// resourceToDimensions will return a set of dimension from the
// resource attributes, including a cloud host id (AWSUniqueId, gcp_id, etc.)
// if it can be constructed from the provided metadata.
func resourceToDimensions(res pdata.Resource) []*sfxpb.Dimension {
	var dims []*sfxpb.Dimension

	if hostID, ok := splunk.ResourceToHostID(res); ok && hostID.Key != splunk.HostIDKeyHost {
		dims = append(dims, &sfxpb.Dimension{
			Key:   string(hostID.Key),
			Value: hostID.ID,
		})
	}

	res.Attributes().ForEach(func(k string, val pdata.AttributeValue) {
		// Never send the SignalFX token
		if k == splunk.SFxAccessTokenLabel {
			return
		}

		dims = append(dims, &sfxpb.Dimension{
			Key:   k,
			Value: tracetranslator.AttributeValueToString(val, false),
		})
	})

	return dims
}

func timestampToSignalFx(ts pdata.TimestampUnixNano) int64 {
	// Convert nanosecs to millisecs.
	return int64(ts) / 1e6
}
