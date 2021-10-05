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

package translation

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"
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
	logger             *zap.Logger
	metricTranslator   *MetricTranslator
	filterSet          *dpfilters.FilterSet
	datapointValidator *datapointValidator
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
	return &MetricsConverter{
		logger:             logger,
		metricTranslator:   t,
		filterSet:          fs,
		datapointValidator: newDatapointValidator(logger, nonAlphanumericDimChars),
	}, nil
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

	return c.datapointValidator.sanitizeDataPoints(sfxDatapoints)
}

func (c *MetricsConverter) metricToSfxDataPoints(metric pdata.Metric, extraDimensions []*sfxpb.Dimension) []*sfxpb.DataPoint {
	// TODO: Figure out some efficient way to know how many datapoints there
	// will be in the given metric.
	var dps []*sfxpb.DataPoint

	basePoint := makeBaseDataPoint(metric)

	switch metric.DataType() {
	case pdata.MetricDataTypeNone:
		return nil
	case pdata.MetricDataTypeGauge:
		dps = convertNumberDatapoints(metric.Gauge().DataPoints(), basePoint, extraDimensions)
	case pdata.MetricDataTypeSum:
		dps = convertNumberDatapoints(metric.Sum().DataPoints(), basePoint, extraDimensions)
	case pdata.MetricDataTypeHistogram:
		dps = convertHistogram(metric.Histogram().DataPoints(), basePoint, extraDimensions)
	case pdata.MetricDataTypeSummary:
		dps = convertSummaryDataPoints(metric.Summary().DataPoints(), metric.Name(), extraDimensions)
	}

	if c.metricTranslator != nil {
		dps = c.metricTranslator.TranslateDataPoints(c.logger, dps)
	}

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

func attributesToDimensions(attributes pdata.AttributeMap, extraDims []*sfxpb.Dimension) []*sfxpb.Dimension {
	dimensions := make([]*sfxpb.Dimension, len(extraDims), attributes.Len()+len(extraDims))
	copy(dimensions, extraDims)
	if attributes.Len() == 0 {
		return dimensions
	}
	dimensionsValue := make([]sfxpb.Dimension, attributes.Len())
	pos := 0
	attributes.Range(func(k string, v pdata.AttributeValue) bool {
		dimensionsValue[pos].Key = k
		dimensionsValue[pos].Value = v.AsString()
		dimensions = append(dimensions, &dimensionsValue[pos])
		pos++
		return true
	})
	return dimensions
}

func convertSummaryDataPoints(
	in pdata.SummaryDataPointSlice,
	name string,
	extraDims []*sfxpb.Dimension,
) []*sfxpb.DataPoint {
	out := make([]*sfxpb.DataPoint, 0, in.Len())

	for i := 0; i < in.Len(); i++ {
		inDp := in.At(i)

		dims := attributesToDimensions(inDp.Attributes(), extraDims)
		ts := timestampToSignalFx(inDp.Timestamp())

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

func convertNumberDatapoints(in pdata.NumberDataPointSlice, basePoint *sfxpb.DataPoint, extraDims []*sfxpb.Dimension) []*sfxpb.DataPoint {
	out := make([]*sfxpb.DataPoint, 0, in.Len())

	for i := 0; i < in.Len(); i++ {
		inDp := in.At(i)

		dp := *basePoint
		dp.Timestamp = timestampToSignalFx(inDp.Timestamp())
		dp.Dimensions = attributesToDimensions(inDp.Attributes(), extraDims)

		switch inDp.Type() {
		case pdata.MetricValueTypeInt:
			val := inDp.IntVal()
			dp.Value.IntValue = &val
		case pdata.MetricValueTypeDouble:
			val := inDp.DoubleVal()
			dp.Value.DoubleValue = &val
		}

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

	case pdata.MetricDataTypeGauge:
		return &sfxMetricTypeGauge

	case pdata.MetricDataTypeSum:
		if !metric.Sum().IsMonotonic() {
			return &sfxMetricTypeGauge
		}
		if metric.Sum().AggregationTemporality() == pdata.MetricAggregationTemporalityDelta {
			return &sfxMetricTypeCounter
		}
		return &sfxMetricTypeCumulativeCounter

	case pdata.MetricDataTypeHistogram:
		if metric.Histogram().AggregationTemporality() == pdata.MetricAggregationTemporalityDelta {
			return &sfxMetricTypeCounter
		}
		return &sfxMetricTypeCumulativeCounter
	}

	return nil
}

func convertHistogram(histDPs pdata.HistogramDataPointSlice, basePoint *sfxpb.DataPoint, extraDims []*sfxpb.Dimension) []*sfxpb.DataPoint {
	var out []*sfxpb.DataPoint

	for i := 0; i < histDPs.Len(); i++ {
		histDP := histDPs.At(i)
		ts := timestampToSignalFx(histDP.Timestamp())

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

	res.Attributes().Range(func(k string, val pdata.AttributeValue) bool {
		// Never send the SignalFX token
		if k == splunk.SFxAccessTokenLabel {
			return true
		}

		dims = append(dims, &sfxpb.Dimension{
			Key:   k,
			Value: val.AsString(),
		})
		return true
	})

	return dims
}

func timestampToSignalFx(ts pdata.Timestamp) int64 {
	// Convert nanosecs to millisecs.
	return int64(ts) / 1e6
}

func (c *MetricsConverter) ConvertDimension(dim string) string {
	res := dim
	if c.metricTranslator != nil {
		res = c.metricTranslator.translateDimension(dim)
	}
	return filterKeyChars(res, c.datapointValidator.nonAlphanumericDimChars)
}

// Values obtained from https://dev.splunk.com/observability/docs/datamodel/ingest#Criteria-for-metric-and-dimension-names-and-values
const (
	maxMetricNameLength     = 256
	maxDimensionNameLength  = 128
	maxDimensionValueLength = 256
)

var (
	invalidMetricNameReason = fmt.Sprintf(
		"metric name longer than %d characters", maxMetricNameLength)
	invalidDimensionNameReason = fmt.Sprintf(
		"dimension name longer than %d characters", maxDimensionNameLength)
	invalidDimensionValueReason = fmt.Sprintf(
		"dimension value longer than %d characters", maxDimensionValueLength)
)

type datapointValidator struct {
	logger                  *zap.Logger
	nonAlphanumericDimChars string
}

func newDatapointValidator(logger *zap.Logger, nonAlphanumericDimChars string) *datapointValidator {
	return &datapointValidator{logger: createSampledLogger(logger), nonAlphanumericDimChars: nonAlphanumericDimChars}
}

// sanitizeDataPoints sanitizes datapoints prior to dispatching them to the backend.
// Datapoints that do not conform to the requirements are removed. This method drops
// datapoints with metric name greater than 256 characters.
func (dpv *datapointValidator) sanitizeDataPoints(dps []*sfxpb.DataPoint) []*sfxpb.DataPoint {
	resultDatapointsLen := 0
	for dpIndex, dp := range dps {
		if dpv.isValidMetricName(dp.Metric) {
			dp.Dimensions = dpv.sanitizeDimensions(dp.Dimensions)
			if resultDatapointsLen < dpIndex {
				dps[resultDatapointsLen] = dp
			}
			resultDatapointsLen++
		}
	}

	// Trim datapoints slice to account for any removed datapoints.
	return dps[:resultDatapointsLen]
}

// sanitizeDimensions replaces all characters unsupported by SignalFx backend
// in metric label keys and with "_" and drops dimensions when the key is greater
// than 128 characters or when value is greater than 256 characters in length.
func (dpv *datapointValidator) sanitizeDimensions(dimensions []*sfxpb.Dimension) []*sfxpb.Dimension {
	resultDimensionsLen := 0
	for dimensionIndex, d := range dimensions {
		if dpv.isValidDimension(d) {
			d.Key = filterKeyChars(d.Key, dpv.nonAlphanumericDimChars)
			if resultDimensionsLen < dimensionIndex {
				dimensions[resultDimensionsLen] = d
			}
			resultDimensionsLen++
		}
	}

	// Trim dimensions slice to account for any removed dimensions.
	return dimensions[:resultDimensionsLen]
}

func (dpv *datapointValidator) isValidMetricName(name string) bool {
	if len(name) > maxMetricNameLength {
		dpv.logger.Warn("dropping datapoint",
			zap.String("reason", invalidMetricNameReason),
			zap.String("metric_name", name),
			zap.Int("metric_name_length", len(name)),
		)
		return false
	}
	return true
}

func (dpv *datapointValidator) isValidDimension(dimension *sfxpb.Dimension) bool {
	return dpv.isValidDimensionName(dimension.Key) && dpv.isValidDimensionValue(dimension.Value, dimension.Key)
}

func (dpv *datapointValidator) isValidDimensionName(name string) bool {
	if len(name) > maxDimensionNameLength {
		dpv.logger.Warn("dropping dimension",
			zap.String("reason", invalidDimensionNameReason),
			zap.String("dimension_name", name),
			zap.Int("dimension_name_length", len(name)),
		)
		return false
	}
	return true
}

func (dpv *datapointValidator) isValidDimensionValue(value, name string) bool {
	if len(value) > maxDimensionValueLength {
		dpv.logger.Warn("dropping dimension",
			zap.String("dimension_name", name),
			zap.String("reason", invalidDimensionValueReason),
			zap.String("dimension_value", value),
			zap.Int("dimension_value_length", len(value)),
		)
		return false
	}
	return true
}

// Copied from https://github.com/open-telemetry/opentelemetry-collector/blob/v0.26.0/exporter/exporterhelper/queued_retry.go#L108
func createSampledLogger(logger *zap.Logger) *zap.Logger {
	if logger.Core().Enabled(zapcore.DebugLevel) {
		// Debugging is enabled. Don't do any sampling.
		return logger
	}

	// Create a logger that samples all messages to 1 per 10 seconds initially,
	// and 1/10000 of messages after that.
	opts := zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewSamplerWithOptions(
			core,
			10*time.Second,
			1,
			10000,
		)
	})
	return logger.WithOptions(opts)
}
