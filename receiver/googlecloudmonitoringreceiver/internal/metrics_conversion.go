// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver/internal"

import (
	"encoding/hex"
	"math"
	"reflect"
	"regexp"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/distribution"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

type MetricsBuilder struct {
	logger *zap.Logger
}

const (
	spanContextTypeURL   = "type.googleapis.com/google.monitoring.v3.SpanContext"
	droppedLabelsTypeURL = "type.googleapis.com/google.monitoring.v3.DroppedLabels"
)

var spanNameRegex = regexp.MustCompile("projects/[^/]*/traces/(?P<traceId>[[:alnum:]]*)/spans/(?P<spanId>[[:alnum:]]*)")

func NewMetricsBuilder(logger *zap.Logger) *MetricsBuilder {
	return &MetricsBuilder{
		logger: logger,
	}
}

func (mb *MetricsBuilder) ConvertGaugeToMetrics(ts *monitoringpb.TimeSeries, m pmetric.Metric) pmetric.Metric {
	m.SetName(ts.GetMetric().GetType())
	m.SetUnit(ts.GetUnit())
	gauge := m.SetEmptyGauge()

	for _, point := range ts.GetPoints() {
		dp := gauge.DataPoints().AppendEmpty()

		// Directly check and set the StartTimestamp if valid
		if point.Interval.StartTime != nil && point.Interval.StartTime.IsValid() {
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(point.Interval.StartTime.AsTime()))
		}

		// Check if EndTime is set and valid
		if point.Interval.EndTime != nil && point.Interval.EndTime.IsValid() {
			dp.SetTimestamp(pcommon.NewTimestampFromTime(point.Interval.EndTime.AsTime()))
		} else {
			mb.logger.Warn("EndTime is invalid for metric:", zap.String("Metric", ts.GetMetric().GetType()))
		}

		switch v := point.Value.Value.(type) {
		case *monitoringpb.TypedValue_DoubleValue:
			dp.SetDoubleValue(v.DoubleValue)
		case *monitoringpb.TypedValue_Int64Value:
			dp.SetIntValue(v.Int64Value)
		default:
			mb.logger.Info("Unhandled metric value type:", zap.Reflect("Type", v))
		}
	}

	return m
}

func (mb *MetricsBuilder) ConvertSumToMetrics(ts *monitoringpb.TimeSeries, m pmetric.Metric) pmetric.Metric {
	m.SetName(ts.GetMetric().GetType())
	m.SetUnit(ts.GetUnit())
	sum := m.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	for _, point := range ts.GetPoints() {
		dp := sum.DataPoints().AppendEmpty()

		// Directly check and set the StartTimestamp if valid
		if point.Interval.StartTime != nil && point.Interval.StartTime.IsValid() {
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(point.Interval.StartTime.AsTime()))
		}

		// Check if EndTime is set and valid
		if point.Interval.EndTime != nil && point.Interval.EndTime.IsValid() {
			dp.SetTimestamp(pcommon.NewTimestampFromTime(point.Interval.EndTime.AsTime()))
		} else {
			mb.logger.Warn("EndTime is invalid for metric:", zap.String("Metric", ts.GetMetric().GetType()))
		}

		switch v := point.Value.Value.(type) {
		case *monitoringpb.TypedValue_DoubleValue:
			dp.SetDoubleValue(v.DoubleValue)
		case *monitoringpb.TypedValue_Int64Value:
			dp.SetIntValue(v.Int64Value)
		default:
			mb.logger.Info("Unhandled metric value type:", zap.Reflect("Type", v))
		}
	}

	return m
}

func (mb *MetricsBuilder) ConvertDeltaToMetrics(ts *monitoringpb.TimeSeries, m pmetric.Metric) pmetric.Metric {
	m.SetName(ts.GetMetric().GetType())
	m.SetUnit(ts.GetUnit())
	sum := m.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	for _, point := range ts.GetPoints() {
		dp := sum.DataPoints().AppendEmpty()

		// Directly check and set the StartTimestamp if valid
		if point.Interval.StartTime != nil && point.Interval.StartTime.IsValid() {
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(point.Interval.StartTime.AsTime()))
		}

		// Check if EndTime is set and valid
		if point.Interval.EndTime != nil && point.Interval.EndTime.IsValid() {
			dp.SetTimestamp(pcommon.NewTimestampFromTime(point.Interval.EndTime.AsTime()))
		} else {
			mb.logger.Warn("EndTime is invalid for metric:", zap.String("Metric", ts.GetMetric().GetType()))
		}

		switch v := point.Value.Value.(type) {
		case *monitoringpb.TypedValue_DoubleValue:
			dp.SetDoubleValue(v.DoubleValue)
		case *monitoringpb.TypedValue_Int64Value:
			dp.SetIntValue(v.Int64Value)
		default:
			mb.logger.Info("Unhandled metric value type:", zap.Reflect("Type", v))
		}
	}

	return m
}

// ConvertDistributionToMetrics converts from Google cloud monitoring distributions to OpenTelemetry histograms.
// See https://cloud.google.com/monitoring/api/ref_v3/rest/v3/TypedValue#Distribution for information on distribution metrics.
func (mb *MetricsBuilder) ConvertDistributionToMetrics(ts *monitoringpb.TimeSeries, m pmetric.Metric) pmetric.Metric {
	m.SetName(ts.GetMetric().GetType())
	m.SetUnit(ts.GetUnit())
	histogram := m.SetEmptyHistogram()
	histogram.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	// Note: Google cloud monitoring distributions use inclusive lower bound and exclusive upper bound (see
	// https://cloud.google.com/monitoring/api/ref_v3/rest/v3/TypedValue#bucketoptions), while OpenTelemetry histograms use
	// exclusive lower bounds and inclusive upper bounds. For the conversion, we are deliberately ignoring this discrepancy in
	// accordance with https://opentelemetry.io/docs/specs/otel/metrics/data-model/#histogram-bucket-inclusivity, quote:
	// > Importers and exporters working with OpenTelemetry Metrics data are meant to disregard this specification when
	// > translating to and from histogram formats that use inclusive lower bounds and exclusive upper bounds.

	metricAttributes := convertDistributionLabels(ts.GetMetric().GetLabels())
	for _, sourceDataPoint := range ts.GetPoints() {
		sourceValue := sourceDataPoint.GetValue()
		if sourceValue == nil {
			mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point, missing value.", zap.String("name", ts.Metric.Type))
			continue
		}
		distributionValue := sourceValue.GetDistributionValue()
		if distributionValue == nil {
			mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point, missing distribution value.", zap.String("name", ts.Metric.Type))
			continue
		}
		bucketOptions := distributionValue.GetBucketOptions()
		if bucketOptions == nil {
			mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point, missing bucket options", zap.String("name", ts.Metric.Type))
			continue
		}
		interval := sourceDataPoint.GetInterval()
		if interval == nil {
			mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point, missing interval", zap.String("name", ts.Metric.Type))
			continue
		}

		targetDataPoint := histogram.DataPoints().AppendEmpty()
		startTime := sourceDataPoint.GetInterval().GetStartTime()
		if startTime != nil && startTime.IsValid() {
			targetDataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime.AsTime()))
		} else {
			mb.logger.Debug("Invalid or absent start time on Google Cloud Monitoring distribution data point:", zap.String("Metric", ts.Metric.Type))
		}
		endTime := sourceDataPoint.GetInterval().GetEndTime()
		if endTime != nil && endTime.IsValid() {
			targetDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(endTime.AsTime()))
		} else {
			mb.logger.Debug("Invalid or absent end time on Google Cloud Monitoring distribution data point:", zap.String("Metric", ts.Metric.Type))
			// fall back to the current time stamp if no timestamp is available on the source data point
			targetDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		}

		metricAttributes.CopyTo(targetDataPoint.Attributes())

		mb.convertExemplars(distributionValue, &targetDataPoint)

		sourceBucketCounts := distributionValue.GetBucketCounts()

		switch bucketOptions.Options.(type) {
		case *distribution.Distribution_BucketOptions_ExplicitBuckets:
			mb.convertDistributionDataPointExplicitBuckets(
				ts.Metric.Type,
				len(sourceBucketCounts),
				bucketOptions.GetExplicitBuckets(),
				&targetDataPoint,
			)
		case *distribution.Distribution_BucketOptions_LinearBuckets:
			mb.convertDistributionDataPointLinearBuckets(
				ts.Metric.Type,
				bucketOptions.GetLinearBuckets(),
				&targetDataPoint,
			)
		case *distribution.Distribution_BucketOptions_ExponentialBuckets:
			mb.convertDistributionDataPointExponentialBuckets(
				ts.Metric.Type,
				bucketOptions.GetExponentialBuckets(),
				&targetDataPoint,
			)
		default:
			mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point: Unsupported bucket option type",
				zap.String("name", ts.Metric.Type),
				zap.String("bucket option type)",
					reflect.TypeOf(bucketOptions.Options).String(),
				))
			continue
		}

		countTotal := uint64(0)
		targetBucketCounts := targetDataPoint.BucketCounts()
		targetBucketCounts.EnsureCapacity(len(sourceBucketCounts))
		for _, bucketCount := range sourceBucketCounts {
			if bucketCount >= 0 {
				targetBucketCounts.Append(uint64(bucketCount))
				countTotal += uint64(bucketCount)
			} else {
				// The source data type is int64, the target type is uint64, so we need to handle negative counts in some way.
				// We normalize negative values to zero, so all other counts are at the correct position in the target data point.
				// (Obviously, bucket counts should never be negative in the source data anyway, so this branch should never be
				// executed.)
				targetBucketCounts.Append(0)
			}
		}
		targetDataPoint.SetCount(countTotal)
	}

	return m
}

func convertDistributionLabels(sourceLabels map[string]string) pcommon.Map {
	metricAttributes := pcommon.NewMap()
	metricAttributes.EnsureCapacity(len(sourceLabels))
	for k, v := range sourceLabels {
		metricAttributes.PutStr(k, v)
	}
	return metricAttributes
}

func (mb *MetricsBuilder) convertDistributionDataPointExplicitBuckets(
	metricType string,
	numberOfSourceBucketCounts int,
	buckets *distribution.Distribution_BucketOptions_Explicit,
	targetDataPoint *pmetric.HistogramDataPoint,
) {
	bounds := buckets.GetBounds()
	if len(bounds) == 0 && numberOfSourceBucketCounts > 0 {
		mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point with explicit bucket, 0 buckets and > 0 counts",
			zap.String("name", metricType),
		)
		return
	}
	targetDataPoint.ExplicitBounds().FromRaw(buckets.GetBounds())
	// Note: There is also an implicit overflow bucket with boundary +Inf.
}

func (mb *MetricsBuilder) convertDistributionDataPointLinearBuckets(
	metricType string,
	buckets *distribution.Distribution_BucketOptions_Linear,
	targetDataPoint *pmetric.HistogramDataPoint,
) {
	numFiniteBuckets := buckets.GetNumFiniteBuckets()
	if numFiniteBuckets < 0 {
		mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point, number of finite buckets not within allowed range (must by >= 0)",
			zap.String("name", metricType),
			zap.Int32("number of finite buckets", numFiniteBuckets),
		)
		return
	}
	offset := buckets.GetOffset()
	if offset < 0 {
		mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point, offset is not within allowed range (must >= 0)",
			zap.String("name", metricType),
			zap.Float64("offset", offset),
		)
		return
	}
	width := buckets.GetWidth()
	if width <= 0 {
		mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point, scale is not within allowed range (must be > 0)",
			zap.String("name", metricType),
			zap.Float64("width", width),
		)
		return
	}

	// See https://cloud.google.com/monitoring/api/ref_v3/rest/v3/TypedValue#linear:
	// There are numFiniteBuckets + 2 (= N) buckets. Bucket i has the following boundaries:
	// Lower bound (1 <= i < N): offset + (width * (i - 1)).
	// Upper bound (0 <= i < N-1): offset + (width * i).

	// bucket 0: [-infinity, offset) and
	// buckets 1 - N-1: [offset + (width * (i - 1)), offset + (width * i))
	targetDataPoint.ExplicitBounds().EnsureCapacity(int(numFiniteBuckets) + 1)
	for i := 0; i <= int(numFiniteBuckets); i++ {
		targetDataPoint.ExplicitBounds().Append(offset + width*float64(i))
	}
	// bucket N: implicit overflow bucket [offset + (width * (N - 1)), +infinity)
}

func (mb *MetricsBuilder) convertDistributionDataPointExponentialBuckets(
	metricType string,
	buckets *distribution.Distribution_BucketOptions_Exponential,
	targetDataPoint *pmetric.HistogramDataPoint,
) {
	// Note: This method converts a distribution with exponential buckets to an OpenTelemetry histogram with explicit bounds.
	// An obvious alternative would be to convert it to an exponential histogram. However, this cannot be done without loss of
	// precision. An OpenTelemetry exponential histograms sets the boundary for bucket i at (2^(2^(-scale)))^i, with scale being
	// an integer. Google Cloud Monitoring exponential buckets use a floating point scale and growth factor and set the boundary
	// at scale * (growthFactor ^ i). Depending on the chosen scale and growth factor, even the best approximation of the
	// Google Cloud Monitoring bucket boundaries might deviate from the actual boundaries significantly. Therefore, we instead
	// convert the exponential distribution to a plain OpenTelemetry histogram with explicit bounds. This choice trades payload
	// size for precision (that is, we avoid loss of precision/information by accepting a larger payload size).
	numFiniteBuckets := buckets.GetNumFiniteBuckets()
	if numFiniteBuckets < 0 {
		mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point, number of finite buckets not within allowed range (must by >= 0)",
			zap.String("name", metricType),
			zap.Int32("number of finite buckets", numFiniteBuckets),
		)
		return
	}
	growthFactor := buckets.GetGrowthFactor()
	if growthFactor <= 1.0 {
		mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point, growth factor is not within allowed range (must by > 1)",
			zap.String("name", metricType),
			zap.Float64("growth factor", growthFactor),
		)
		return
	}
	scale := buckets.GetScale()
	if scale <= 0 {
		mb.logger.Debug("Cannot transform Google Cloud Monitoring distribution data point, scale is not within allowed range (must be > 0)",
			zap.String("name", metricType),
			zap.Float64("scale", scale),
		)
		return
	}

	// See https://cloud.google.com/monitoring/api/ref_v3/rest/v3/TypedValue#exponential:
	// There are numFiniteBuckets + 2 (= N) buckets. Bucket i has the following boundaries:
	// Lower bound (1 <= i < N): scale * (growthFactor ^ (i - 1)).
	// Upper bound (0 <= i < N-1): scale * (growthFactor ^ i).

	// bucket 0: [-infinity, offset) and
	// buckets 1 - N-1: [offset + (width * (i - 1)), offset + (width * i))
	targetDataPoint.ExplicitBounds().EnsureCapacity(int(numFiniteBuckets) + 1)
	for i := 0; i <= int(numFiniteBuckets); i++ {
		targetDataPoint.ExplicitBounds().Append(scale * math.Pow(growthFactor, float64(i)))
	}
	// bucket N: implicit overflow bucket [offset + (width * (N - 1)), +infinity)
}

func (mb *MetricsBuilder) convertExemplars(distributionValue *distribution.Distribution, targetDataPoint *pmetric.HistogramDataPoint) {
	sourceExemplars := distributionValue.GetExemplars()
	if len(sourceExemplars) > 0 {
		targetDataPoint.Exemplars().EnsureCapacity(len(sourceExemplars))
		for _, sourceExemplar := range sourceExemplars {
			targetExemplarDataPoint := targetDataPoint.Exemplars().AppendEmpty()
			targetExemplarDataPoint.SetTimestamp(pcommon.NewTimestampFromTime(sourceExemplar.GetTimestamp().AsTime()))
			targetExemplarDataPoint.SetDoubleValue(sourceExemplar.GetValue())
			sourceAttachments := sourceExemplar.GetAttachments()
			for _, sourceAttachment := range sourceAttachments {
				if sourceAttachment == nil {
					continue
				}
				value := sourceAttachment.GetValue()
				if len(value) == 0 {
					// skip attachments with nil or empty value
					continue
				}
				typeURL := sourceAttachment.GetTypeUrl()
				switch typeURL {
				case spanContextTypeURL:
					mb.convertDistributionExemplarAttachmentSpanContext(value, &targetExemplarDataPoint)
				case droppedLabelsTypeURL:
					mb.convertDistributionExemplarAttachmentDroppedLabels(sourceAttachment, &targetExemplarDataPoint)
				default:
					mb.logger.Debug("Discarding exemplar attachment with unsupported type URL", zap.String("type URL", typeURL))
				}
			}
		}
	}
}

func (mb *MetricsBuilder) convertDistributionExemplarAttachmentSpanContext(sourceValueBytes []byte, targetExemplarDataPoint *pmetric.Exemplar) {
	matches := spanNameRegex.FindSubmatch(sourceValueBytes)
	if matches == nil || len(matches) != 3 {
		mb.logger.Debug(
			"Failed to parse span context from Google Cloud Monitoring distribution data point exemplar attachment, value did not match the expected format "+spanNameRegex.String(),
			zap.String("attachment value", string(sourceValueBytes)),
		)
		return
	}
	traceIDRaw := matches[1]
	spanIDRaw := matches[2]
	traceID := make([]byte, 16)
	spanID := make([]byte, 8)
	_, err := hex.Decode(traceID, traceIDRaw)
	if err != nil {
		mb.logger.Debug(
			"Failed to convert trace ID from span context of from Google Cloud Monitoring distribution data point exemplar attachment",
			zap.String("attachment value", string(sourceValueBytes)),
			zap.String("parsed trace ID", string(traceIDRaw)),
			zap.Error(err),
		)
	} else {
		targetExemplarDataPoint.SetTraceID(pcommon.TraceID(traceID))
	}
	_, err = hex.Decode(spanID, spanIDRaw)
	if err != nil {
		mb.logger.Debug(
			"Failed to convert span ID from span context of from Google Cloud Monitoring distribution data point exemplar attachment",
			zap.String("attachment value", string(sourceValueBytes)),
			zap.String("parsed span ID", string(spanIDRaw)),
			zap.Error(err),
		)
	} else {
		targetExemplarDataPoint.SetSpanID(pcommon.SpanID(spanID))
	}
}

func (mb *MetricsBuilder) convertDistributionExemplarAttachmentDroppedLabels(sourceValue *anypb.Any, targetExemplarDataPoint *pmetric.Exemplar) {
	sourceDroppedLabels := monitoringpb.DroppedLabels{}
	if err := anypb.UnmarshalTo(sourceValue, &sourceDroppedLabels, proto.UnmarshalOptions{}); err != nil {
		mb.logger.Debug(
			"Failed to convert dropped labels from Google Cloud Monitoring distribution data point exemplar attachment",
			zap.Error(err),
		)
		return
	}
	targetDroppedLabels := make(map[string]any, len(sourceDroppedLabels.Label))
	for k, v := range sourceDroppedLabels.Label {
		targetDroppedLabels[k] = v
	}
	if err := targetExemplarDataPoint.FilteredAttributes().PutEmptyMap(droppedLabelsTypeURL).FromRaw(targetDroppedLabels); err != nil {
		mb.logger.Debug(
			"Failed to set dropped labels in Google Cloud Monitoring distribution data point exemplar",
			zap.Error(err),
		)
	}
}
