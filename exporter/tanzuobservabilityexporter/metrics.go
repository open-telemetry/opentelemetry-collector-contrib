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
	"strconv"
	"sync/atomic"

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

// metricsConsumer instances consume OTEL metrics
type metricsConsumer struct {
	consumerMap           map[pdata.MetricDataType]typedMetricConsumer
	sender                flushCloser
	reportInternalMetrics bool
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
		ilms := rms.At(i).InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ms := ilms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				m := ms.At(k)
				select {
				case <-ctx.Done():
					return multierr.Combine(append(errs, errors.New("context canceled"))...)
				default:
					c.pushSingleMetric(m, &errs)
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

func (c *metricsConsumer) pushSingleMetric(m pdata.Metric, errs *[]error) {
	dataType := m.DataType()
	consumer := c.consumerMap[dataType]
	if consumer == nil {
		*errs = append(
			*errs, fmt.Errorf("no support for metric type %v", dataType))

	} else {
		consumer.Consume(m, errs)
	}
}

// typedMetricConsumer consumes one specific type of OTEL metric
type typedMetricConsumer interface {

	// Type returns the type of metric this consumer consumes. For example
	// Gauge, Sum, or Histogram
	Type() pdata.MetricDataType

	// Consume consumes the metric and appends any errors encountered to errs
	Consume(m pdata.Metric, errs *[]error)

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
	switch numberDataPoint.Type() {
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
	metric pdata.Metric,
	numberDataPoint pdata.NumberDataPoint,
	errs *[]error,
	sender gaugeSender,
	settings component.TelemetrySettings,
	missingValues *counter,
) {
	tags := attributesToTags(numberDataPoint.Attributes())
	ts := numberDataPoint.Timestamp().AsTime().Unix()
	value, err := getValue(numberDataPoint)
	if err != nil {
		logMissingValue(metric, settings, missingValues)
		return
	}
	err = sender.SendMetric(metric.Name(), value, ts, "", tags)
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

func (g *gaugeConsumer) Consume(metric pdata.Metric, errs *[]error) {
	gauge := metric.Gauge()
	numberDataPoints := gauge.DataPoints()
	for i := 0; i < numberDataPoints.Len(); i++ {
		pushGaugeNumberDataPoint(
			metric,
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

func (s *sumConsumer) Consume(metric pdata.Metric, errs *[]error) {
	sum := metric.Sum()
	isDelta := sum.AggregationTemporality() == pdata.MetricAggregationTemporalityDelta
	numberDataPoints := sum.DataPoints()
	for i := 0; i < numberDataPoints.Len(); i++ {
		// If sum metric is a delta type, send it to tanzu observability as a
		// delta counter. Otherwise, send it to tanzu observability as a gauge
		// metric.
		if isDelta {
			s.pushNumberDataPoint(metric, numberDataPoints.At(i), errs)
		} else {
			pushGaugeNumberDataPoint(
				metric, numberDataPoints.At(i), errs, s.sender, s.settings, &s.missingValues)
		}
	}
}

func (s *sumConsumer) PushInternalMetrics(errs *[]error) {
	s.missingValues.Report(missingValueMetricName, typeIsSumTags, s.sender, errs)
}

func (s *sumConsumer) pushNumberDataPoint(
	metric pdata.Metric, numberDataPoint pdata.NumberDataPoint, errs *[]error,
) {
	tags := attributesToTags(numberDataPoint.Attributes())
	value, err := getValue(numberDataPoint)
	if err != nil {
		logMissingValue(metric, s.settings, &s.missingValues)
		return
	}
	err = s.sender.SendDeltaCounter(metric.Name(), value, "", tags)
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
}

// newHistogramConsumer returns a metricConsumer that consumes histograms.
// cumulative and delta handle cumulative and delta histograms respectively.
// sender sends internal metrics to wavefront.
func newHistogramConsumer(
	cumulative, delta histogramDataPointConsumer,
	sender gaugeSender,
	settings component.TelemetrySettings,
) typedMetricConsumer {
	return &histogramConsumer{
		cumulative: cumulative,
		delta:      delta,
		sender:     sender,
		reporting:  newHistogramReporting(settings),
	}
}

func (h *histogramConsumer) Type() pdata.MetricDataType {
	return pdata.MetricDataTypeHistogram
}

func (h *histogramConsumer) Consume(metric pdata.Metric, errs *[]error) {
	histogram := metric.Histogram()
	aggregationTemporality := histogram.AggregationTemporality()
	var consumer histogramDataPointConsumer
	switch aggregationTemporality {
	case pdata.MetricAggregationTemporalityDelta:
		// TODO: follow-on pull request will add support for Delta Histograms.
		*errs = append(*errs, errors.New("delta histograms not supported"))
		return
	case pdata.MetricAggregationTemporalityCumulative:
		consumer = h.cumulative
	default:
		h.reporting.LogNoAggregationTemporality(metric)
		return
	}
	histogramDataPoints := histogram.DataPoints()
	for i := 0; i < histogramDataPoints.Len(); i++ {
		consumer.Consume(metric, histogramDataPoints.At(i), errs, h.reporting)
	}
}

func (h *histogramConsumer) PushInternalMetrics(errs *[]error) {
	h.reporting.Report(h.sender, errs)
}

// histogramDataPointConsumer consumes one histogram data point. There is one
// implementation for delta histograms and one for cumulative histograms.
type histogramDataPointConsumer interface {

	// Consume consumes the histogram data point.
	// metric is the enclosing metric; histogram is the histogram data point;
	// errors get appended to errs; reporting keeps track of special situations
	Consume(
		metric pdata.Metric,
		histogram pdata.HistogramDataPoint,
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
	metric pdata.Metric,
	h pdata.HistogramDataPoint,
	errs *[]error,
	reporting *histogramReporting,
) {
	name := metric.Name()
	tags := attributesToTags(h.Attributes())
	ts := h.Timestamp().AsTime().Unix()
	explicitBounds := h.ExplicitBounds()
	bucketCounts := h.BucketCounts()
	if len(bucketCounts) != len(explicitBounds)+1 {
		reporting.LogMalformed(metric)
		return
	}
	if leTag, ok := tags["le"]; ok {
		tags["_le"] = leTag
	}
	var leCount uint64
	for i := range bucketCounts {
		tags["le"] = leTagValue(explicitBounds, i)
		leCount += bucketCounts[i]
		err := c.sender.SendMetric(name, float64(leCount), ts, "", tags)
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
