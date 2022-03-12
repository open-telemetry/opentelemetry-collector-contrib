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
	"go.opentelemetry.io/collector/model/pdata"
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

var (
	typeIsGaugeTags     = map[string]string{"type": "gauge"}
	typeIsSumTags       = map[string]string{"type": "sum"}
	typeIsHistogramTags = map[string]string{"type": "histogram"}
)

var (
	allGranularity = map[histogram.Granularity]bool{histogram.DAY: true, histogram.HOUR: true, histogram.MINUTE: true}
)

var (
	// Specifies regular histogram
	regularHistogram histogramConsumerSpec = regularHistogramConsumerSpec{}

	// Specifies exponential histograms
	exponentialHistogram histogramConsumerSpec = exponentialHistogramConsumerSpec{}
)

// metricsConsumer instances consume OTEL metrics
type metricsConsumer struct {
	consumerMap           map[pdata.MetricDataType]typedMetricConsumer
	sender                flushCloser
	reportInternalMetrics bool
}

type metricInfo struct {
	pdata.Metric
	Source string
	Tags   map[string]string
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
) *metricsConsumer {
	consumerMap := make(map[pdata.MetricDataType]typedMetricConsumer, len(consumers))
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
	}
}

// Consume consumes OTEL metrics. For each metric in md, it delegates to the
// typedMetricConsumer that consumes that type of metric. Once Consume consumes
// all the metrics, it calls Flush() on the sender passed to
// newMetricsConsumer.
func (c *metricsConsumer) Consume(ctx context.Context, md pdata.Metrics) error {
	var errs []error
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i).Resource().Attributes()
		source, resourceTags := getSourceAndResourceTags(rm)
		ilms := rms.At(i).InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ms := ilms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				mi := metricInfo{m, source, resourceTags}
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
	dataType := mi.DataType()
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
	Type() pdata.MetricDataType

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

// counter represents an internal counter metric. The zero value is ready to use
type counter struct {
	count int64
}

// Report reports this counter to tanzu observability. name is the name of
// the metric to be reported. tags is the tags for the metric. sender is what
// sends the metric to tanzu observability. Any errors get added to errs.
func (c *counter) Report(
	name string, tags map[string]string, sender gaugeSender, errs *[]error,
) {
	err := sender.SendMetric(name, float64(c.Get()), 0, "", tags)
	if err != nil {
		*errs = append(*errs, err)
	}
}

// Inc increments this counter by one.
func (c *counter) Inc() {
	atomic.AddInt64(&c.count, 1)
}

// Get gets the value of this counter.
func (c *counter) Get() int64 {
	return atomic.LoadInt64(&c.count)
}

// logMissingValue keeps track of metrics with missing values. metric is the
// metric with the missing value. settings logs the missing value. count counts
// metrics with missing values.
func logMissingValue(metric pdata.Metric, settings component.TelemetrySettings, count *counter) {
	namef := zap.String(metricNameString, metric.Name())
	typef := zap.String(metricTypeString, metric.DataType().String())
	settings.Logger.Debug("Metric missing value", namef, typef)
	count.Inc()
}

// getValue gets the floating point value out of a NumberDataPoint
func getValue(numberDataPoint pdata.NumberDataPoint) (float64, error) {
	switch numberDataPoint.ValueType() {
	case pdata.MetricValueTypeInt:
		return float64(numberDataPoint.IntVal()), nil
	case pdata.MetricValueTypeDouble:
		return numberDataPoint.DoubleVal(), nil
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
	numberDataPoint pdata.NumberDataPoint,
	errs *[]error,
	sender gaugeSender,
	settings component.TelemetrySettings,
	missingValues *counter,
) {
	tags := attributesToTags(mi.Tags, numberDataPoint.Attributes())
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
	missingValues counter
}

// newGaugeConsumer returns a typedMetricConsumer that consumes gauge metrics
// by sending them to tanzu observability.
func newGaugeConsumer(
	sender gaugeSender, settings component.TelemetrySettings) typedMetricConsumer {
	return &gaugeConsumer{
		sender:   sender,
		settings: settings,
	}
}

func (g *gaugeConsumer) Type() pdata.MetricDataType {
	return pdata.MetricDataTypeGauge
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
			&g.missingValues)
	}
}

func (g *gaugeConsumer) PushInternalMetrics(errs *[]error) {
	g.missingValues.Report(missingValueMetricName, typeIsGaugeTags, g.sender, errs)
}

type sumConsumer struct {
	sender        senders.MetricSender
	settings      component.TelemetrySettings
	missingValues counter
}

// newSumConsumer returns a typedMetricConsumer that consumes sum metrics
// by sending them to tanzu observability.
func newSumConsumer(
	sender senders.MetricSender, settings component.TelemetrySettings) typedMetricConsumer {
	return &sumConsumer{
		sender:   sender,
		settings: settings,
	}
}

func (s *sumConsumer) Type() pdata.MetricDataType {
	return pdata.MetricDataTypeSum
}

func (s *sumConsumer) Consume(mi metricInfo, errs *[]error) {
	sum := mi.Sum()
	isDelta := sum.AggregationTemporality() == pdata.MetricAggregationTemporalityDelta
	numberDataPoints := sum.DataPoints()
	for i := 0; i < numberDataPoints.Len(); i++ {
		// If sum is a delta type, send it to tanzu observability as a
		// delta counter. Otherwise, send it to tanzu observability as a gauge
		// metric.
		if isDelta {
			s.pushNumberDataPoint(mi, numberDataPoints.At(i), errs)
		} else {
			pushGaugeNumberDataPoint(
				mi, numberDataPoints.At(i), errs, s.sender, s.settings, &s.missingValues)
		}
	}
}

func (s *sumConsumer) PushInternalMetrics(errs *[]error) {
	s.missingValues.Report(missingValueMetricName, typeIsSumTags, s.sender, errs)
}

func (s *sumConsumer) pushNumberDataPoint(mi metricInfo, numberDataPoint pdata.NumberDataPoint, errs *[]error) {
	tags := attributesToTags(mi.Tags, numberDataPoint.Attributes())
	value, err := getValue(numberDataPoint)
	if err != nil {
		logMissingValue(mi.Metric, s.settings, &s.missingValues)
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
	malformedHistograms      counter
	noAggregationTemporality counter
}

// newHistogramReporting returns a new histogramReporting instance.
func newHistogramReporting(settings component.TelemetrySettings) *histogramReporting {
	return &histogramReporting{settings: settings}
}

// Malformed returns the number of malformed histogram data points.
func (r *histogramReporting) Malformed() int64 {
	return r.malformedHistograms.Get()
}

// NoAggregationTemporality returns the number of histogram metrics that have no
// aggregation temporality.
func (r *histogramReporting) NoAggregationTemporality() int64 {
	return r.noAggregationTemporality.Get()
}

// LogMalformed logs seeing one malformed data point.
func (r *histogramReporting) LogMalformed(metric pdata.Metric) {
	namef := zap.String(metricNameString, metric.Name())
	r.settings.Logger.Debug("Malformed histogram", namef)
	r.malformedHistograms.Inc()
}

// LogNoAggregationTemporality logs seeing a histogram metric with no aggregation temporality
func (r *histogramReporting) LogNoAggregationTemporality(metric pdata.Metric) {
	namef := zap.String(metricNameString, metric.Name())
	r.settings.Logger.Debug("histogram metric missing aggregation temporality", namef)
	r.noAggregationTemporality.Inc()
}

// Report sends the counts in this instance to wavefront.
// sender is what sends to wavefront. Any errors sending get added to errs.
func (r *histogramReporting) Report(sender gaugeSender, errs *[]error) {
	r.malformedHistograms.Report(malformedHistogramMetricName, nil, sender, errs)
	r.noAggregationTemporality.Report(
		noAggregationTemporalityMetricName,
		typeIsHistogramTags,
		sender,
		errs)
}

type histogramConsumer struct {
	cumulative histogramDataPointConsumer
	delta      histogramDataPointConsumer
	sender     gaugeSender
	reporting  *histogramReporting
	spec       histogramConsumerSpec
}

// newHistogramConsumer returns a metricConsumer that consumes histograms.
// cumulative and delta handle cumulative and delta histograms respectively.
// sender sends internal metrics to wavefront.
func newHistogramConsumer(
	cumulative, delta histogramDataPointConsumer,
	sender gaugeSender,
	spec histogramConsumerSpec,
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

func (h *histogramConsumer) Type() pdata.MetricDataType {
	return h.spec.Type()
}

func (h *histogramConsumer) Consume(mi metricInfo, errs *[]error) {
	aHistogram := h.spec.AsHistogram(mi.Metric)
	aggregationTemporality := aHistogram.AggregationTemporality()
	var consumer histogramDataPointConsumer
	switch aggregationTemporality {
	case pdata.MetricAggregationTemporalityDelta:
		consumer = h.delta
	case pdata.MetricAggregationTemporalityCumulative:
		consumer = h.cumulative
	default:
		h.reporting.LogNoAggregationTemporality(mi.Metric)
		return
	}
	length := aHistogram.Len()
	for i := 0; i < length; i++ {
		consumer.Consume(mi, aHistogram.At(i), errs, h.reporting)
	}
}

func (h *histogramConsumer) PushInternalMetrics(errs *[]error) {
	h.reporting.Report(h.sender, errs)
}

// histogramDataPointConsumer consumes one histogram data point. There is one
// implementation for delta histograms and one for cumulative histograms.
type histogramDataPointConsumer interface {

	// Consume consumes the histogram data point.
	// mi is the metricInfo which encloses metric; histogram is the histogram data point;
	// errors get appended to errs; reporting keeps track of special situations
	Consume(
		mi metricInfo,
		histogram histogramDataPoint,
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
	histogram histogramDataPoint,
	errs *[]error,
	reporting *histogramReporting,
) {
	name := mi.Name()
	tags := attributesToTags(mi.Tags, histogram.Attributes())
	ts := histogram.Timestamp().AsTime().Unix()
	explicitBounds := histogram.ExplicitBounds()
	bucketCounts := histogram.BucketCounts()
	if len(bucketCounts) != len(explicitBounds)+1 {
		reporting.LogMalformed(mi.Metric)
		return
	}
	if leTag, ok := tags["le"]; ok {
		tags["_le"] = leTag
	}
	var leCount uint64
	for i := range bucketCounts {
		tags["le"] = leTagValue(explicitBounds, i)
		leCount += bucketCounts[i]
		err := c.sender.SendMetric(name, float64(leCount), ts, mi.Source, tags)
		if err != nil {
			*errs = append(*errs, err)
		}
	}
}

func leTagValue(explicitBounds []float64, bucketIndex int) string {
	if bucketIndex == len(explicitBounds) {
		return "+Inf"
	}
	return strconv.FormatFloat(explicitBounds[bucketIndex], 'f', -1, 64)
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
	his histogramDataPoint,
	errs *[]error,
	reporting *histogramReporting) {
	name := mi.Name()
	tags := attributesToTags(mi.Tags, his.Attributes())
	ts := his.Timestamp().AsTime().Unix()
	explicitBounds := his.ExplicitBounds()
	bucketCounts := his.BucketCounts()
	if len(bucketCounts) != len(explicitBounds)+1 {
		reporting.LogMalformed(mi.Metric)
		return
	}
	centroids := make([]histogram.Centroid, len(bucketCounts))
	for i := range bucketCounts {
		centroids[i] = histogram.Centroid{
			Value: centroidValue(explicitBounds, i), Count: int(bucketCounts[i])}
	}
	err := d.sender.SendDistribution(name, centroids, allGranularity, ts, mi.Source, tags)
	if err != nil {
		*errs = append(*errs, err)
	}
}

func centroidValue(explicitBounds []float64, index int) float64 {
	length := len(explicitBounds)
	if length == 0 {
		// This is the best we can do.
		return 0.0
	}
	if index == 0 {
		return explicitBounds[0]
	}
	if index == length {
		return explicitBounds[length-1]
	}
	return (explicitBounds[index-1] + explicitBounds[index]) / 2.0
}

// histogramDataPoint represents either a regular or exponential histogram data point
type histogramDataPoint interface {
	Count() uint64
	ExplicitBounds() []float64
	BucketCounts() []uint64
	Attributes() pdata.AttributeMap
	Timestamp() pdata.Timestamp
}

// histogramMetric represents either a regular or exponential histogram
type histogramMetric interface {

	// AggregationTemporality returns whether the histogram is delta or cumulative
	AggregationTemporality() pdata.MetricAggregationTemporality

	// Len returns the number of data points in this histogram
	Len() int

	// At returns the ith histogramDataPoint where 0 is the first.
	At(i int) histogramDataPoint
}

// histogramConsumerSpec is the specification for either regular or exponential histograms
type histogramConsumerSpec interface {

	// Type returns either regular or exponential histogram
	Type() pdata.MetricDataType

	// AsHistogram returns given metric as a regular or exponential histogram depending on
	// what Type returns.
	AsHistogram(metric pdata.Metric) histogramMetric
}

type regularHistogramMetric struct {
	pdata.Histogram
	pdata.HistogramDataPointSlice
}

func (r *regularHistogramMetric) At(i int) histogramDataPoint {
	return r.HistogramDataPointSlice.At(i)
}

type regularHistogramConsumerSpec struct {
}

func (regularHistogramConsumerSpec) Type() pdata.MetricDataType {
	return pdata.MetricDataTypeHistogram
}

func (regularHistogramConsumerSpec) AsHistogram(metric pdata.Metric) histogramMetric {
	aHistogram := metric.Histogram()
	return &regularHistogramMetric{
		Histogram:               aHistogram,
		HistogramDataPointSlice: aHistogram.DataPoints(),
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

func (s *summaryConsumer) Type() pdata.MetricDataType {
	return pdata.MetricDataTypeSummary
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
	mi metricInfo, summaryDataPoint pdata.SummaryDataPoint, errs *[]error,
) {
	name := mi.Name()
	ts := summaryDataPoint.Timestamp().AsTime().Unix()
	tags := attributesToTags(mi.Tags, summaryDataPoint.Attributes())
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

type exponentialHistogramDataPoint struct {
	pdata.ExponentialHistogramDataPoint
	bucketCounts   []uint64
	explicitBounds []float64
}

// newExponentialHistogram converts a pdata.ExponentialHistogramDataPoint into a histogramDataPoint
// implementation. A regular histogramDataPoint has bucket counts and explicit bounds for each
// bucket; an ExponentialHistogramDataPoint has only bucket counts because the explicit bounds
// for each bucket are implied because they grow exponentially from bucket to bucket. The
// conversion of an ExponentialHistogramDataPoint to a histogramDataPoint is necessary because the
// code that sends histograms to tanzuobservability expects the histogramDataPoint format.
func newExponentialHistogramDataPoint(dataPoint pdata.ExponentialHistogramDataPoint) histogramDataPoint {

	// Base is the factor by which the explicit bounds increase from bucket to bucket.
	// This formula comes from the documentation here:
	// https://github.com/open-telemetry/opentelemetry-proto/blob/8ba33cceb4a6704af68a4022d17868a7ac1d94f4/opentelemetry/proto/metrics/v1/metrics.proto#L487
	base := math.Pow(2.0, math.Pow(2.0, -float64(dataPoint.Scale())))

	// ExponentialHistogramDataPoints have buckets with negative explicit bounds, buckets with
	// positive explicit bounds, and a "zero" bucket. Our job is to merge these bucket groups into
	// a single list of buckets and explicit bounds.
	negativeBucketCounts := dataPoint.Negative().BucketCounts()
	positiveBucketCounts := dataPoint.Positive().BucketCounts()

	// The total number of buckets is the number of negative buckets + the number of positive
	// buckets + 1 for the zero bucket + 1 bucket for negative infinity up to the negative explicit
	// bound with largest magnitude + 1 bucket for the largest positive explicit bound up to
	// positive infinity.
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
		dataPoint.Negative().Offset(), base, negativeBucketCounts, &bucketCounts, &explicitBounds)
	appendZeroBucketAndExplicitBound(
		dataPoint.Positive().Offset(), base, dataPoint.ZeroCount(), &bucketCounts, &explicitBounds)
	appendPositiveBucketsAndExplicitBounds(
		dataPoint.Positive().Offset(), base, positiveBucketCounts, &bucketCounts, &explicitBounds)
	return &exponentialHistogramDataPoint{
		ExponentialHistogramDataPoint: dataPoint,
		bucketCounts:                  bucketCounts,
		explicitBounds:                explicitBounds,
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

func (e *exponentialHistogramDataPoint) ExplicitBounds() []float64 {
	return e.explicitBounds
}

func (e *exponentialHistogramDataPoint) BucketCounts() []uint64 {
	return e.bucketCounts
}

type exponentialHistogramMetric struct {
	pdata.ExponentialHistogram
	pdata.ExponentialHistogramDataPointSlice
}

func (e *exponentialHistogramMetric) At(i int) histogramDataPoint {
	return newExponentialHistogramDataPoint(e.ExponentialHistogramDataPointSlice.At(i))
}

type exponentialHistogramConsumerSpec struct {
}

func (exponentialHistogramConsumerSpec) Type() pdata.MetricDataType {
	return pdata.MetricDataTypeExponentialHistogram
}

func (exponentialHistogramConsumerSpec) AsHistogram(metric pdata.Metric) histogramMetric {
	aHistogram := metric.ExponentialHistogram()
	return &exponentialHistogramMetric{
		ExponentialHistogram:               aHistogram,
		ExponentialHistogramDataPointSlice: aHistogram.DataPoints(),
	}
}
