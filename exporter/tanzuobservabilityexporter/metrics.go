// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tanzuobservabilityexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter"

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"

	"github.com/wavefronthq/wavefront-sdk-go/histogram"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	missingValueMetricName             = "~sdk.otel.collector.missing_values"
	metricNameString                   = "metric name"
	metricTypeString                   = "metric type"
	malformedHistogramMetricName       = "~sdk.otel.collector.malformed_histogram"
	noAggregationTemporalityMetricName = "~sdk.otel.collector.no_aggregation_temporality"
)

const (
	histogramDataPointInvalid = "Histogram data point invalid"
)

var (
	typeIsGaugeTags     = map[string]string{"type": "gauge"}
	typeIsSumTags       = map[string]string{"type": "sum"}
	typeIsHistogramTags = map[string]string{"type": "histogram"}
)

var (
	allGranularity = map[histogram.Granularity]bool{histogram.DAY: true, histogram.HOUR: true, histogram.MINUTE: true}
)

var (
	regularHistogram     histogramSpecification = regularHistogramSpecification{}
	exponentialHistogram histogramSpecification = exponentialHistogramSpecification{}
)

// metricsConsumer instances consume OTEL metrics
type metricsConsumer struct {
	consumerMap           map[pmetric.MetricType]typedMetricConsumer
	sender                flushCloser
	reportInternalMetrics bool
	config                MetricsConfig
}

type metricInfo struct {
	pmetric.Metric
	Source        string
	SourceKey     string
	ResourceAttrs map[string]string
}

// newMetricsConsumer returns a new metricsConsumer. consumers are the
// consumers responsible for consuming each type of metric. The Consume method
// of returned consumer calls the Flush method on sender after consuming
// all the metrics. Calling Close on the returned metricsConsumer calls Close
// on sender. sender can be nil.  reportInternalMetrics controls whether
// returned metricsConsumer reports internal metrics.
func newMetricsConsumer(
	consumers []typedMetricConsumer,
	sender flushCloser,
	reportInternalMetrics bool,
	config MetricsConfig,
) *metricsConsumer {
	consumerMap := make(map[pmetric.MetricType]typedMetricConsumer, len(consumers))
	for _, consumer := range consumers {
		if consumerMap[consumer.Type()] != nil {
			panic("duplicate consumer type detected: " + consumer.Type().String())
		}
		consumerMap[consumer.Type()] = consumer
	}
	return &metricsConsumer{
		consumerMap:           consumerMap,
		sender:                sender,
		reportInternalMetrics: reportInternalMetrics,
		config:                config,
	}
}

// Consume consumes OTEL metrics. For each metric in md, it delegates to the
// typedMetricConsumer that consumes that type of metric. Once Consume consumes
// all the metrics, it calls Flush() on the sender passed to
// newMetricsConsumer.
func (c *metricsConsumer) Consume(ctx context.Context, md pmetric.Metrics) error {
	var errs []error
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		resAttrs := rms.At(i).Resource().Attributes()
		source, sourceKey := getSourceAndKey(resAttrs)
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ms := ilms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				var resAttrsMap map[string]string
				if c.config.ResourceAttrsIncluded {
					resAttrsMap = attributesToTags(resAttrs)
				} else if !c.config.AppTagsExcluded {
					resAttrsMap = appAttributesToTags(resAttrs)
				}
				mi := metricInfo{Metric: m, Source: source, SourceKey: sourceKey, ResourceAttrs: resAttrsMap}
				select {
				case <-ctx.Done():
					return multierr.Combine(append(errs, errors.New("context canceled"))...)
				default:
					c.pushSingleMetric(mi, &errs)
				}
			}
		}
	}
	if c.reportInternalMetrics {
		c.pushInternalMetrics(&errs)
	}
	if c.sender != nil {
		if err := c.sender.Flush(); err != nil {
			errs = append(errs, err)
		}
	}
	return multierr.Combine(errs...)
}

// Close closes this metricsConsumer by calling Close on the sender passed
// to newMetricsConsumer.
func (c *metricsConsumer) Close() {
	if c.sender != nil {
		c.sender.Close()
	}
}

func (c *metricsConsumer) pushInternalMetrics(errs *[]error) {
	for _, consumer := range c.consumerMap {
		consumer.PushInternalMetrics(errs)
	}
}

func (c *metricsConsumer) pushSingleMetric(mi metricInfo, errs *[]error) {
	dataType := mi.Type()
	consumer := c.consumerMap[dataType]
	if consumer == nil {
		*errs = append(
			*errs, fmt.Errorf("no support for metric type %v", dataType))

	} else {
		consumer.Consume(mi, errs)
	}
}

// typedMetricConsumer consumes one specific type of OTEL metric
type typedMetricConsumer interface {

	// Type returns the type of metric this consumer consumes. For example
	// Gauge, Sum, or Histogram
	Type() pmetric.MetricType

	// Consume consumes the metric from the metricInfo and appends any errors encountered to errs
	Consume(mi metricInfo, errs *[]error)

	// PushInternalMetrics sends internal metrics for this consumer to tanzu observability
	// and appends any errors encountered to errs. The Consume method of metricsConsumer calls
	// PushInternalMetrics on each registered typedMetricConsumer after it has consumed all the
	// metrics but before it calls Flush on the sender.
	PushInternalMetrics(errs *[]error)
}

// flushCloser is the interface for the Flush and Close method
type flushCloser interface {
	Flush() error
	Close()
}

// report the counter to tanzu observability. name is the name of
// the metric to be reported. tags is the tags for the metric. sender is what
// sends the metric to tanzu observability. Any errors get added to errs.
func report(count *atomic.Int64, name string, tags map[string]string, sender gaugeSender, errs *[]error) {
	err := sender.SendMetric(name, float64(count.Load()), 0, "", tags)
	if err != nil {
		*errs = append(*errs, err)
	}
}

// logMissingValue keeps track of metrics with missing values. metric is the
// metric with the missing value. settings logs the missing value. count counts
// metrics with missing values.
func logMissingValue(metric pmetric.Metric, settings component.TelemetrySettings, count *atomic.Int64) {
	namef := zap.String(metricNameString, metric.Name())
	typef := zap.String(metricTypeString, metric.Type().String())
	settings.Logger.Debug("Metric missing value", namef, typef)
	count.Add(1)
}

// getValue gets the floating point value out of a NumberDataPoint
func getValue(numberDataPoint pmetric.NumberDataPoint) (float64, error) {
	switch numberDataPoint.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		return float64(numberDataPoint.IntValue()), nil
	case pmetric.NumberDataPointValueTypeDouble:
		return numberDataPoint.DoubleValue(), nil
	default:
		return 0.0, errors.New("unsupported metric value type")
	}
}

// pushGaugeNumberDataPoint sends a metric as a gauge metric to tanzu
// observability. metric is the metric to send. numberDataPoint is the value
// of the metric. Any errors get appended to errs. sender is what sends the
// gauge metric to tanzu observability. settings logs problems. missingValues
// keeps track of metrics with missing values.
func pushGaugeNumberDataPoint(
	mi metricInfo,
	numberDataPoint pmetric.NumberDataPoint,
	errs *[]error,
	sender gaugeSender,
	settings component.TelemetrySettings,
	missingValues *atomic.Int64,
) {
	tags := pointAndResAttrsToTagsAndFixSource(mi.SourceKey, numberDataPoint.Attributes(), newMap(mi.ResourceAttrs))
	ts := numberDataPoint.Timestamp().AsTime().Unix()
	value, err := getValue(numberDataPoint)
	if err != nil {
		logMissingValue(mi.Metric, settings, missingValues)
		return
	}
	err = sender.SendMetric(mi.Name(), value, ts, mi.Source, tags)
	if err != nil {
		*errs = append(*errs, err)
	}
}

// gaugeSender sends gauge metrics to tanzu observability
type gaugeSender interface {
	SendMetric(name string, value float64, ts int64, source string, tags map[string]string) error
}

type gaugeConsumer struct {
	sender        gaugeSender
	settings      component.TelemetrySettings
	missingValues *atomic.Int64
}

// newGaugeConsumer returns a typedMetricConsumer that consumes gauge metrics
// by sending them to tanzu observability.
func newGaugeConsumer(
	sender gaugeSender, settings component.TelemetrySettings) typedMetricConsumer {
	return &gaugeConsumer{
		sender:        sender,
		settings:      settings,
		missingValues: &atomic.Int64{},
	}
}

func (g *gaugeConsumer) Type() pmetric.MetricType {
	return pmetric.MetricTypeGauge
}

func (g *gaugeConsumer) Consume(mi metricInfo, errs *[]error) {
	gauge := mi.Gauge()
	numberDataPoints := gauge.DataPoints()
	for i := 0; i < numberDataPoints.Len(); i++ {
		pushGaugeNumberDataPoint(
			mi,
			numberDataPoints.At(i),
			errs,
			g.sender,
			g.settings,
			g.missingValues)
	}
}

func (g *gaugeConsumer) PushInternalMetrics(errs *[]error) {
	report(g.missingValues, missingValueMetricName, typeIsGaugeTags, g.sender, errs)
}

type sumConsumer struct {
	sender        senders.MetricSender
	settings      component.TelemetrySettings
	missingValues *atomic.Int64
}

// newSumConsumer returns a typedMetricConsumer that consumes sum metrics
// by sending them to tanzu observability.
func newSumConsumer(
	sender senders.MetricSender, settings component.TelemetrySettings) typedMetricConsumer {
	return &sumConsumer{
		sender:        sender,
		settings:      settings,
		missingValues: &atomic.Int64{},
	}
}

func (s *sumConsumer) Type() pmetric.MetricType {
	return pmetric.MetricTypeSum
}

func (s *sumConsumer) Consume(mi metricInfo, errs *[]error) {
	sum := mi.Sum()
	isDelta := sum.AggregationTemporality() == pmetric.AggregationTemporalityDelta
	numberDataPoints := sum.DataPoints()
	for i := 0; i < numberDataPoints.Len(); i++ {
		// If sum is a delta type, send it to tanzu observability as a
		// delta counter. Otherwise, send it to tanzu observability as a gauge
		// metric.
		if isDelta {
			s.pushNumberDataPoint(mi, numberDataPoints.At(i), errs)
		} else {
			pushGaugeNumberDataPoint(
				mi, numberDataPoints.At(i), errs, s.sender, s.settings, s.missingValues)
		}
	}
}

func (s *sumConsumer) PushInternalMetrics(errs *[]error) {
	report(s.missingValues, missingValueMetricName, typeIsSumTags, s.sender, errs)
}

func (s *sumConsumer) pushNumberDataPoint(mi metricInfo, numberDataPoint pmetric.NumberDataPoint, errs *[]error) {
	tags := pointAndResAttrsToTagsAndFixSource(mi.SourceKey, numberDataPoint.Attributes(), newMap(mi.ResourceAttrs))
	value, err := getValue(numberDataPoint)
	if err != nil {
		logMissingValue(mi.Metric, s.settings, s.missingValues)
		return
	}
	err = s.sender.SendDeltaCounter(mi.Name(), value, mi.Source, tags)
	if err != nil {
		*errs = append(*errs, err)
	}
}

// histogramReporting takes care of logging and internal metrics for histograms
type histogramReporting struct {
	settings                 component.TelemetrySettings
	malformedHistograms      *atomic.Int64
	noAggregationTemporality *atomic.Int64
}

// newHistogramReporting returns a new histogramReporting instance.
func newHistogramReporting(settings component.TelemetrySettings) *histogramReporting {
	return &histogramReporting{
		settings:                 settings,
		malformedHistograms:      &atomic.Int64{},
		noAggregationTemporality: &atomic.Int64{},
	}
}

// Malformed returns the number of malformed histogram data points.
func (r *histogramReporting) Malformed() int64 {
	return r.malformedHistograms.Load()
}

// NoAggregationTemporality returns the number of histogram metrics that have no
// aggregation temporality.
func (r *histogramReporting) NoAggregationTemporality() int64 {
	return r.noAggregationTemporality.Load()
}

// LogMalformed logs seeing one malformed data point.
func (r *histogramReporting) LogMalformed(metric pmetric.Metric) {
	namef := zap.String(metricNameString, metric.Name())
	r.settings.Logger.Debug("Malformed histogram", namef)
	r.malformedHistograms.Add(1)
}

// LogNoAggregationTemporality logs seeing a histogram metric with no aggregation temporality
func (r *histogramReporting) LogNoAggregationTemporality(metric pmetric.Metric) {
	namef := zap.String(metricNameString, metric.Name())
	r.settings.Logger.Debug("histogram metric missing aggregation temporality", namef)
	r.noAggregationTemporality.Add(1)
}

// Report sends the counts in this instance to wavefront.
// sender is what sends to wavefront. Any errors sending get added to errs.
func (r *histogramReporting) Report(sender gaugeSender, errs *[]error) {
	report(r.malformedHistograms, malformedHistogramMetricName, nil, sender, errs)
	report(r.noAggregationTemporality, noAggregationTemporalityMetricName, typeIsHistogramTags, sender, errs)
}

type histogramConsumer struct {
	cumulative histogramDataPointConsumer
	delta      histogramDataPointConsumer
	sender     gaugeSender
	reporting  *histogramReporting
	spec       histogramSpecification
}

// newHistogramConsumer returns a metricConsumer that consumes histograms.
// cumulative and delta handle cumulative and delta histograms respectively.
// sender sends internal metrics to wavefront.
func newHistogramConsumer(
	cumulative, delta histogramDataPointConsumer,
	sender gaugeSender,
	spec histogramSpecification,
	settings component.TelemetrySettings,
) typedMetricConsumer {
	return &histogramConsumer{
		cumulative: cumulative,
		delta:      delta,
		sender:     sender,
		reporting:  newHistogramReporting(settings),
		spec:       spec,
	}
}

func (h *histogramConsumer) Type() pmetric.MetricType {
	return h.spec.Type()
}

func (h *histogramConsumer) Consume(mi metricInfo, errs *[]error) {
	aggregationTemporality := h.spec.AggregationTemporality(mi.Metric)
	var consumer histogramDataPointConsumer
	switch aggregationTemporality {
	case pmetric.AggregationTemporalityDelta:
		consumer = h.delta
	case pmetric.AggregationTemporalityCumulative:
		consumer = h.cumulative
	default:
		h.reporting.LogNoAggregationTemporality(mi.Metric)
		return
	}
	points := h.spec.DataPoints(mi.Metric)
	for _, point := range points {
		consumer.Consume(mi, point, errs, h.reporting)
	}
}

func (h *histogramConsumer) PushInternalMetrics(errs *[]error) {
	h.reporting.Report(h.sender, errs)
}

// histogramDataPointConsumer consumes one histogram data point. There is one
// implementation for delta histograms and one for cumulative histograms.
type histogramDataPointConsumer interface {

	// Consume consumes a BucketHistogramDataPoint.
	// mi is the metricInfo which encloses metric; point is the BucketHistogramDataPoint;
	// errors get appended to errs; reporting keeps track of special situations
	Consume(
		mi metricInfo,
		point bucketHistogramDataPoint,
		errs *[]error,
		reporting *histogramReporting,
	)
}

type cumulativeHistogramDataPointConsumer struct {
	sender gaugeSender
}

// newCumulativeHistogramDataPointConsumer returns a consumer for cumulative
// histogram data points.
func newCumulativeHistogramDataPointConsumer(sender gaugeSender) histogramDataPointConsumer {
	return &cumulativeHistogramDataPointConsumer{sender: sender}
}

func (c *cumulativeHistogramDataPointConsumer) Consume(
	mi metricInfo,
	point bucketHistogramDataPoint,
	errs *[]error,
	reporting *histogramReporting,
) {
	if !point.Valid() {
		reporting.LogMalformed(mi.Metric)
		return
	}
	name := mi.Name()
	tags := pointAndResAttrsToTagsAndFixSource(mi.SourceKey, point.Attributes, newMap(mi.ResourceAttrs))
	if leTag, ok := tags["le"]; ok {
		tags["_le"] = leTag
	}
	buckets := point.AsCumulative()
	for _, bucket := range buckets {
		tags["le"] = bucket.Tag
		err := c.sender.SendMetric(
			name, float64(bucket.Count), point.SecondsSinceEpoch, mi.Source, tags)
		if err != nil {
			*errs = append(*errs, err)
		}
	}
}

type deltaHistogramDataPointConsumer struct {
	sender senders.DistributionSender
}

// newDeltaHistogramDataPointConsumer returns a consumer for delta
// histogram data points.
func newDeltaHistogramDataPointConsumer(
	sender senders.DistributionSender) histogramDataPointConsumer {
	return &deltaHistogramDataPointConsumer{sender: sender}
}

func (d *deltaHistogramDataPointConsumer) Consume(
	mi metricInfo,
	point bucketHistogramDataPoint,
	errs *[]error,
	reporting *histogramReporting) {
	if !point.Valid() {
		reporting.LogMalformed(mi.Metric)
		return
	}
	name := mi.Name()
	tags := pointAndResAttrsToTagsAndFixSource(mi.SourceKey, point.Attributes, newMap(mi.ResourceAttrs))
	err := d.sender.SendDistribution(
		name, point.AsDelta(), allGranularity, point.SecondsSinceEpoch, mi.Source, tags)
	if err != nil {
		*errs = append(*errs, err)
	}
}

type summaryConsumer struct {
	sender   gaugeSender
	settings component.TelemetrySettings
}

// newSummaryConsumer returns a typedMetricConsumer that consumes summary metrics
// by sending them to tanzu observability.
func newSummaryConsumer(
	sender gaugeSender, settings component.TelemetrySettings,
) typedMetricConsumer {
	return &summaryConsumer{sender: sender, settings: settings}
}

func (s *summaryConsumer) Type() pmetric.MetricType {
	return pmetric.MetricTypeSummary
}

func (s *summaryConsumer) Consume(mi metricInfo, errs *[]error) {
	summary := mi.Summary()
	summaryDataPoints := summary.DataPoints()
	for i := 0; i < summaryDataPoints.Len(); i++ {
		s.sendSummaryDataPoint(mi, summaryDataPoints.At(i), errs)
	}
}

// PushInternalMetrics is here so that summaryConsumer implements typedMetricConsumer
func (*summaryConsumer) PushInternalMetrics(*[]error) {
	// Do nothing
}

func (s *summaryConsumer) sendSummaryDataPoint(
	mi metricInfo, summaryDataPoint pmetric.SummaryDataPoint, errs *[]error,
) {
	name := mi.Name()
	ts := summaryDataPoint.Timestamp().AsTime().Unix()
	tags := pointAndResAttrsToTagsAndFixSource(mi.SourceKey, summaryDataPoint.Attributes(), newMap(mi.ResourceAttrs))
	count := summaryDataPoint.Count()
	sum := summaryDataPoint.Sum()

	if quantileTag, ok := tags["quantile"]; ok {
		tags["_quantile"] = quantileTag
		delete(tags, "quantile")
	}
	s.sendMetric(name+"_count", float64(count), ts, tags, errs, mi.Source)
	s.sendMetric(name+"_sum", sum, ts, tags, errs, mi.Source)
	quantileValues := summaryDataPoint.QuantileValues()
	for i := 0; i < quantileValues.Len(); i++ {
		quantileValue := quantileValues.At(i)
		tags["quantile"] = quantileTagValue(quantileValue.Quantile())
		s.sendMetric(name, quantileValue.Value(), ts, tags, errs, mi.Source)
	}
}

func (s *summaryConsumer) sendMetric(
	name string,
	value float64,
	ts int64,
	tags map[string]string,
	errs *[]error,
	source string) {
	err := s.sender.SendMetric(name, value, ts, source, tags)
	if err != nil {
		*errs = append(*errs, err)
	}
}

func quantileTagValue(quantile float64) string {
	return strconv.FormatFloat(quantile, 'f', -1, 64)
}

// cumulativeBucket represents a cumulative histogram bucket
type cumulativeBucket struct {

	// The value of the "le" tag
	Tag string

	// The count of values less than or equal to the "le" tag
	Count uint64
}

// bucketHistogramDataPoint represents a single histogram data point
type bucketHistogramDataPoint struct {
	Attributes        pcommon.Map
	SecondsSinceEpoch int64

	// The bucket counts. For exponential histograms, the first and last element of bucketCounts
	// are always 0.
	bucketCounts []uint64

	// The explicit bounds len(explicitBounds) + 1 == len(bucketCounts)
	// If explicitBounds = {10, 20} and bucketCounts = {1, 2, 3} it means that 1 value is <= 10;
	// 2 values are between 10 and 20; and 3 values are > 20
	explicitBounds []float64

	// true if data point came from an exponential histogram.
	exponential bool
}

// Valid returns true if this is a valid data point.
func (b *bucketHistogramDataPoint) Valid() bool {
	return len(b.bucketCounts) == len(b.explicitBounds)+1
}

// AsCumulative returns the buckets for a cumulative histogram
func (b *bucketHistogramDataPoint) AsCumulative() []cumulativeBucket {
	if !b.Valid() {
		panic(histogramDataPointInvalid)
	}

	// For exponential histograms, we ignore the first bucket which always has count 0
	// but include the last bucket for +Inf.
	if b.exponential {
		return b.asCumulative(1, len(b.bucketCounts))
	}
	return b.asCumulative(0, len(b.bucketCounts))
}

// AsDelta returns the centroids for a delta histogram
func (b *bucketHistogramDataPoint) AsDelta() []histogram.Centroid {
	if !b.Valid() {
		panic(histogramDataPointInvalid)
	}

	// For exponential histograms, we ignore the first and last centroids which always have a
	// count of 0.
	if b.exponential {
		return b.asDelta(1, len(b.bucketCounts)-1)
	}
	return b.asDelta(0, len(b.bucketCounts))
}

func (b *bucketHistogramDataPoint) asCumulative(
	startBucketIndex, endBucketIndex int) []cumulativeBucket {
	result := make([]cumulativeBucket, 0, endBucketIndex-startBucketIndex)
	var leCount uint64
	for i := startBucketIndex; i < endBucketIndex; i++ {
		leCount += b.bucketCounts[i]
		result = append(result, cumulativeBucket{Tag: b.leTagValue(i), Count: leCount})
	}
	return result
}

func (b *bucketHistogramDataPoint) asDelta(
	startBucketIndex, endBucketIndex int) []histogram.Centroid {
	result := make([]histogram.Centroid, 0, endBucketIndex-startBucketIndex)
	for i := startBucketIndex; i < endBucketIndex; i++ {
		result = append(
			result,
			histogram.Centroid{Value: b.centroidValue(i), Count: int(b.bucketCounts[i])})
	}
	return result
}

func (b *bucketHistogramDataPoint) leTagValue(bucketIndex int) string {
	if bucketIndex == len(b.explicitBounds) {
		return "+Inf"
	}
	return strconv.FormatFloat(b.explicitBounds[bucketIndex], 'f', -1, 64)
}

func (b *bucketHistogramDataPoint) centroidValue(index int) float64 {
	length := len(b.explicitBounds)
	if length == 0 {
		// This is the best we can do.
		return 0.0
	}
	if index == 0 {
		return b.explicitBounds[0]
	}
	if index == length {
		return b.explicitBounds[length-1]
	}
	return (b.explicitBounds[index-1] + b.explicitBounds[index]) / 2.0
}

type histogramSpecification interface {
	Type() pmetric.MetricType
	AggregationTemporality(metric pmetric.Metric) pmetric.AggregationTemporality
	DataPoints(metric pmetric.Metric) []bucketHistogramDataPoint
}

type regularHistogramSpecification struct {
}

func (regularHistogramSpecification) Type() pmetric.MetricType {
	return pmetric.MetricTypeHistogram
}

func (regularHistogramSpecification) AggregationTemporality(
	metric pmetric.Metric) pmetric.AggregationTemporality {
	return metric.Histogram().AggregationTemporality()
}

func (regularHistogramSpecification) DataPoints(metric pmetric.Metric) []bucketHistogramDataPoint {
	return fromOtelHistogram(metric.Histogram().DataPoints())
}

type exponentialHistogramSpecification struct {
}

func (exponentialHistogramSpecification) Type() pmetric.MetricType {
	return pmetric.MetricTypeExponentialHistogram
}

func (exponentialHistogramSpecification) AggregationTemporality(
	metric pmetric.Metric) pmetric.AggregationTemporality {
	return metric.ExponentialHistogram().AggregationTemporality()
}

func (exponentialHistogramSpecification) DataPoints(
	metric pmetric.Metric) []bucketHistogramDataPoint {
	return fromOtelExponentialHistogram(metric.ExponentialHistogram().DataPoints())
}

// fromOtelHistogram converts a regular histogram metric into a slice of data points.
func fromOtelHistogram(points pmetric.HistogramDataPointSlice) []bucketHistogramDataPoint {
	result := make([]bucketHistogramDataPoint, points.Len())
	for i := 0; i < points.Len(); i++ {
		result[i] = fromOtelHistogramDataPoint(points.At(i))
	}
	return result
}

// fromOtelExponentialHistogram converts an exponential histogram into a slice of data points.
func fromOtelExponentialHistogram(
	points pmetric.ExponentialHistogramDataPointSlice) []bucketHistogramDataPoint {
	result := make([]bucketHistogramDataPoint, points.Len())
	for i := 0; i < points.Len(); i++ {
		result[i] = fromOtelExponentialHistogramDataPoint(points.At(i))
	}
	return result
}

func fromOtelHistogramDataPoint(point pmetric.HistogramDataPoint) bucketHistogramDataPoint {
	return bucketHistogramDataPoint{
		Attributes:        point.Attributes(),
		SecondsSinceEpoch: point.Timestamp().AsTime().Unix(),
		bucketCounts:      point.BucketCounts().AsRaw(),
		explicitBounds:    point.ExplicitBounds().AsRaw(),
	}
}

func fromOtelExponentialHistogramDataPoint(
	point pmetric.ExponentialHistogramDataPoint) bucketHistogramDataPoint {

	// Base is the factor by which the explicit bounds increase from bucket to bucket.
	// This formula comes from the documentation here:
	// https://github.com/open-telemetry/opentelemetry-proto/blob/8ba33cceb4a6704af68a4022d17868a7ac1d94f4/opentelemetry/proto/metrics/v1/metrics.proto#L487
	base := math.Pow(2.0, math.Pow(2.0, -float64(point.Scale())))

	// ExponentialHistogramDataPoints have buckets with negative explicit bounds, buckets with
	// positive explicit bounds, and a "zero" bucket. Our job is to merge these bucket groups into
	// a single list of buckets and explicit bounds.
	negativeBucketCounts := point.Negative().BucketCounts().AsRaw()
	positiveBucketCounts := point.Positive().BucketCounts().AsRaw()

	// The total number of buckets is the number of negative buckets + the number of positive
	// buckets + 1 for the zero bucket + 1 bucket for negative infinity up to the smallest negative explicit bound
	// + 1 bucket for the largest positive explicit bound up to positive infinity.
	numBucketCounts := 1 + len(negativeBucketCounts) + 1 + len(positiveBucketCounts) + 1

	// We pre-allocate the slice setting its length to 0 so that GO doesn't have to keep
	// re-allocating the slice as it grows.
	bucketCounts := make([]uint64, 0, numBucketCounts)

	// The number of explicit bounds is always 1 less than the number of buckets. This is how
	// explicit bounds work. If you have 2 explicit bounds say {2.0, 5.0} then you have 3 buckets:
	// one for values less than 2.0; one for values between 2.0 and 5.0; and one for values greater
	// than 5.0.
	explicitBounds := make([]float64, 0, numBucketCounts-1)

	appendNegativeBucketsAndExplicitBounds(
		point.Negative().Offset(), base, negativeBucketCounts, &bucketCounts, &explicitBounds)
	appendZeroBucketAndExplicitBound(
		point.Positive().Offset(), base, point.ZeroCount(), &bucketCounts, &explicitBounds)
	appendPositiveBucketsAndExplicitBounds(
		point.Positive().Offset(), base, positiveBucketCounts, &bucketCounts, &explicitBounds)
	return bucketHistogramDataPoint{
		Attributes:        point.Attributes(),
		SecondsSinceEpoch: point.Timestamp().AsTime().Unix(),
		bucketCounts:      bucketCounts,
		explicitBounds:    explicitBounds,
		exponential:       true,
	}
}

// appendNegativeBucketsAndExplicitBounds appends negative buckets and explicit bounds to
// bucketCounts and explicitBounds respectively. The largest negative explicit bound (the one
// with the smallest magnitude) is -1*base^negativeOffset
func appendNegativeBucketsAndExplicitBounds(
	negativeOffset int32,
	base float64,
	negativeBucketCounts []uint64,
	bucketCounts *[]uint64,
	explicitBounds *[]float64,
) {
	// The count in the first bucket which includes negative infinity is always 0.
	*bucketCounts = append(*bucketCounts, 0)

	// The smallest negative explicit bound.
	le := -math.Pow(base, float64(negativeOffset)+float64(len(negativeBucketCounts)))
	*explicitBounds = append(*explicitBounds, le)

	// The first negativeBucketCount has a negative explicit bound with the smallest magnitude;
	// the last negativeBucketCount has a negative explicit bound with the largest magnitude.
	// Therefore, to go in order from smallest to largest explicit bound, we have to start with
	// the last element in the negativeBucketCounts array.
	for i := len(negativeBucketCounts) - 1; i >= 0; i-- {
		*bucketCounts = append(*bucketCounts, negativeBucketCounts[i])
		le /= base // We divide by base because our explicit bounds are getting larger as we go
		*explicitBounds = append(*explicitBounds, le)
	}
}

// appendZeroBucketAndExplicitBound appends the "zero" bucket and explicit bound to bucketCounts
// and explicitBounds respectively. The smallest positive explicit bound is base^positiveOffset.
func appendZeroBucketAndExplicitBound(
	positiveOffset int32,
	base float64,
	zeroBucketCount uint64,
	bucketCounts *[]uint64,
	explicitBounds *[]float64,
) {
	*bucketCounts = append(*bucketCounts, zeroBucketCount)

	// The explicit bound of the zeroBucketCount is the smallest positive explicit bound
	*explicitBounds = append(*explicitBounds, math.Pow(base, float64(positiveOffset)))
}

// appendPositiveBucketsAndExplicitBounds appends positive buckets and explicit bounds to
// bucketCounts and explicitBounds respectively. The smallest positive explicit bound is
// base^positiveOffset.
func appendPositiveBucketsAndExplicitBounds(
	positiveOffset int32,
	base float64,
	positiveBucketCounts []uint64,
	bucketCounts *[]uint64,
	explicitBounds *[]float64,
) {
	le := math.Pow(base, float64(positiveOffset))
	for _, bucketCount := range positiveBucketCounts {
		*bucketCounts = append(*bucketCounts, bucketCount)
		le *= base
		*explicitBounds = append(*explicitBounds, le)
	}
	// Last bucket count for positive infinity is always 0.
	*bucketCounts = append(*bucketCounts, 0)
}
