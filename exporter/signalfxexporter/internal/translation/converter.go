// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"

import (
	"fmt"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"
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
	logger               *zap.Logger
	metricTranslator     *MetricTranslator
	filterSet            *dpfilters.FilterSet
	datapointValidator   *datapointValidator
	translator           *signalfx.FromTranslator
	dropHistogramBuckets bool
	processHistograms    bool
}

// NewMetricsConverter creates a MetricsConverter from the passed in logger and
// MetricTranslator. Pass in a nil MetricTranslator to not use translation
// rules.
func NewMetricsConverter(
	logger *zap.Logger,
	t *MetricTranslator,
	excludes []dpfilters.MetricFilter,
	includes []dpfilters.MetricFilter,
	nonAlphanumericDimChars string,
	dropHistogramBuckets bool,
	processHistograms bool,
) (*MetricsConverter, error) {
	fs, err := dpfilters.NewFilterSet(excludes, includes)
	if err != nil {
		return nil, err
	}
	return &MetricsConverter{
		logger:               logger,
		metricTranslator:     t,
		filterSet:            fs,
		datapointValidator:   newDatapointValidator(logger, nonAlphanumericDimChars),
		translator:           &signalfx.FromTranslator{},
		dropHistogramBuckets: dropHistogramBuckets,
		processHistograms:    processHistograms,
	}, nil
}

func (c *MetricsConverter) Start() {
	if c.metricTranslator != nil {
		c.metricTranslator.Start()
	}
}

// MetricsToSignalFxV2 converts the passed in MetricsData to SFx datapoints
// and if processHistograms is set, histogram metrics are not converted to SFx format.
// It returns those datapoints and the number of time series that had to be
// dropped because of errors or warnings.
func (c *MetricsConverter) MetricsToSignalFxV2(md pmetric.Metrics) []*sfxpb.DataPoint {
	var sfxDataPoints []*sfxpb.DataPoint
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		extraDimensions := resourceToDimensions(rm.Resource())

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			var initialDps []*sfxpb.DataPoint
			for k := 0; k < ilm.Metrics().Len(); k++ {
				currentMetric := ilm.Metrics().At(k)
				dps := c.translator.FromMetric(currentMetric, extraDimensions, c.dropHistogramBuckets, c.processHistograms)
				initialDps = append(initialDps, dps...)
			}

			// Translate and filter all metrics within the current ScopeMetric
			sfxDataPoints = append(sfxDataPoints, c.translateAndFilter(initialDps)...)
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
			c.logger.Debug("Datapoint does not match filter, skipping", zap.Stringer("dp", dp))
		}
	}
	dps = dps[:resultSliceLen]
	return dps
}

// resourceToDimensions will return a set of dimension from the
// resource attributes, including a cloud host id (AWSUniqueId, gcp_id, etc.)
// if it can be constructed from the provided metadata.
func resourceToDimensions(res pcommon.Resource) []*sfxpb.Dimension {
	var dims []*sfxpb.Dimension

	if hostID, ok := splunk.ResourceToHostID(res); ok && hostID.Key != splunk.HostIDKeyHost {
		dims = append(dims, &sfxpb.Dimension{
			Key:   string(hostID.Key),
			Value: hostID.ID,
		})
	}

	for k, val := range res.Attributes().All() {
		// Never send the SignalFX token
		if k == splunk.SFxAccessTokenLabel {
			continue
		}

		dims = append(dims, &sfxpb.Dimension{
			Key:   k,
			Value: val.AsString(),
		})
	}

	return dims
}

func (c *MetricsConverter) Shutdown() {
	if c.metricTranslator != nil {
		c.metricTranslator.Shutdown()
	}
}

// Values obtained from https://dev.splunk.com/observability/docs/datamodel/ingest#Criteria-for-metric-and-dimension-names-and-values
const (
	maxMetricNameLength     = 256
	maxDimensionNameLength  = 128
	maxDimensionValueLength = 256
	maxNumberOfDimensions   = 36
)

var (
	invalidMetricNameReason = fmt.Sprintf(
		"metric name longer than %d characters", maxMetricNameLength,
	)
	invalidDimensionNameReason = fmt.Sprintf(
		"dimension name longer than %d characters", maxDimensionNameLength,
	)
	invalidDimensionValueReason = fmt.Sprintf(
		"dimension value longer than %d characters", maxDimensionValueLength,
	)
	invalidNumberOfDimensions = fmt.Sprintf(
		"number of dimensions is larger than %d", maxNumberOfDimensions,
	)
)

type datapointValidator struct {
	logger                  *zap.Logger
	nonAlphanumericDimChars string
}

func newDatapointValidator(logger *zap.Logger, nonAlphanumericDimChars string) *datapointValidator {
	return &datapointValidator{logger: logger, nonAlphanumericDimChars: nonAlphanumericDimChars}
}

// sanitizeDataPoints logs a debug message for any datapoint that violates SignalFx backend
// constraints on metric name length, number of dimensions, dimension name length, or
// dimension value length. These datapoints are no longer dropped by the exporter, since
// the backend already enforces these constraints and drops offending datapoints at ingest.
func (dpv *datapointValidator) sanitizeDataPoints(dps []*sfxpb.DataPoint) []*sfxpb.DataPoint {
	for _, dp := range dps {
		dpv.logIfInvalid(dp)
		dpv.sanitizeDimensionKeys(dp.Dimensions)
	}
	return dps
}

// sanitizeDimensionKeys replaces all characters unsupported by the SignalFx backend
// in dimension keys with "_".
func (dpv *datapointValidator) sanitizeDimensionKeys(dims []*sfxpb.Dimension) {
	for _, d := range dims {
		d.Key = dimensions.FilterKeyChars(d.Key, dpv.nonAlphanumericDimChars)
	}
}

// logIfInvalid logs a single debug message listing every distinct constraint a datapoint violates.
func (dpv *datapointValidator) logIfInvalid(dp *sfxpb.DataPoint) {
	var (
		reasons             []string
		dimNameReasonAdded  bool
		dimValueReasonAdded bool
	)

	if len(dp.Metric) > maxMetricNameLength {
		reasons = append(reasons, invalidMetricNameReason)
	}
	if len(dp.Dimensions) > maxNumberOfDimensions {
		reasons = append(reasons, invalidNumberOfDimensions)
	}
	for _, d := range dp.Dimensions {
		if len(d.Key) > maxDimensionNameLength && !dimNameReasonAdded {
			dimNameReasonAdded = true
			reasons = append(reasons, invalidDimensionNameReason)
		}
		if len(d.Value) > maxDimensionValueLength && !dimValueReasonAdded {
			dimValueReasonAdded = true
			reasons = append(reasons, invalidDimensionValueReason)
		}
	}

	if len(reasons) == 0 {
		return
	}
	dpv.logger.Debug("datapoint is not valid and will be dropped at ingest",
		zap.Strings("reasons", reasons),
		zap.Stringer("datapoint", dp),
	)
}
