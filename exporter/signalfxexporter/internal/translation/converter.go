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

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"

import (
	"fmt"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx"
)

// Some fields on SignalFx protobuf are pointers, in order to reduce
// allocations create the most used ones.
var (
	// SignalFx metric types used in the conversions.
	sfxMetricTypeGauge             = sfxpb.MetricType_GAUGE
	sfxMetricTypeCumulativeCounter = sfxpb.MetricType_CUMULATIVE_COUNTER
	sfxMetricTypeCounter           = sfxpb.MetricType_COUNTER
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

// MetricsToSignalFxV2 converts the passed in MetricsData to SFx datapoints,
// returning those datapoints and the number of time series that had to be
// dropped because of errors or warnings.
func (c *MetricsConverter) MetricsToSignalFxV2(md pdata.Metrics) []*sfxpb.DataPoint {
	var sfxDataPoints []*sfxpb.DataPoint

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		extraDimensions := resourceToDimensions(rm.Resource())

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				dps := signalfx.FromMetric(ilm.Metrics().At(k), extraDimensions)
				dps = c.translateAndFilter(dps)
				sfxDataPoints = append(sfxDataPoints, dps...)
			}
		}
	}

	return c.datapointValidator.sanitizeDataPoints(sfxDataPoints)
}

func (c *MetricsConverter) translateAndFilter(dps []*sfxpb.DataPoint) []*sfxpb.DataPoint {
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
		} else {
			c.logger.Debug("Datapoint does not match filter, skipping", zap.String("dp", DatapointToString(dp)))
		}
	}
	dps = dps[:resultSliceLen]
	return dps
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

	res.Attributes().Range(func(k string, val pdata.Value) bool {
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

func DatapointToString(dp *sfxpb.DataPoint) string {
	var tsStr string
	if dp.Timestamp != 0 {
		tsStr = strconv.FormatInt(dp.Timestamp, 10)
	}

	var dimsStr string
	for _, dim := range dp.Dimensions {
		dimsStr = dimsStr + dim.String()
	}

	return fmt.Sprintf("%s: %s (%s) %s\n%s", dp.Metric, dp.Value.String(), dpTypeToString(*dp.MetricType), tsStr, dimsStr)
}

func dpTypeToString(t sfxpb.MetricType) string {
	switch t {
	case sfxpb.MetricType_GAUGE:
		return "Gauge"
	case sfxpb.MetricType_COUNTER:
		return "Counter"
	case sfxpb.MetricType_ENUM:
		return "Enum"
	case sfxpb.MetricType_CUMULATIVE_COUNTER:
		return "Cumulative Counter"
	default:
		return fmt.Sprintf("unsupported type %d", t)
	}
}
