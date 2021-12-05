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
	"sync/atomic"

	"github.com/wavefronthq/wavefront-sdk-go/senders"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	missingValueMetricName = "~sdk.otel.collector.missing_values"
	metricNameString       = "metric name"
	metricTypeString       = "metric type"
)

var (
	typeIsGaugeTags = map[string]string{"type": "gauge"}
	typeIsSumTags   = map[string]string{"type": "sum"}
)

// metricsConsumer instances consume OTEL metrics
type metricsConsumer struct {
	consumerMap map[pdata.MetricDataType]typedMetricConsumer
	sender      flushCloser
}

// newMetricsConsumer returns a new metricsConsumer. consumers are the
// consumers responsible for consuming each type metric. The Consume method
// of returned consumer calls the Flush method on sender after consuming
// all the metrics. Calling Close on the returned metricsConsumer calls Close
// on sender. sender can be nil.
func newMetricsConsumer(consumers []typedMetricConsumer, sender flushCloser) *metricsConsumer {
	consumerMap := make(map[pdata.MetricDataType]typedMetricConsumer, len(consumers))
	for _, consumer := range consumers {
		if consumerMap[consumer.Type()] != nil {
			panic("duplicate consumer type detected: " + consumer.Type().String())
		}
		consumerMap[consumer.Type()] = consumer
	}
	return &metricsConsumer{
		consumerMap: consumerMap,
		sender:      sender,
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
	c.pushInternalMetrics(&errs)
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
// metric with the missing value. logger is the logger. count counts
// metrics with missing values.
func logMissingValue(metric pdata.Metric, logger *zap.Logger, count *counter) {
	namef := zap.String(metricNameString, metric.Name())
	typef := zap.String(metricTypeString, metric.DataType().String())
	logger.Debug("Metric missing value", namef, typef)
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
// gauge metric to tanzu observability. logger is the logger. missingValues
// keeps track of metrics with missing values.
func pushGaugeNumberDataPoint(
	metric pdata.Metric,
	numberDataPoint pdata.NumberDataPoint,
	errs *[]error,
	sender gaugeSender,
	logger *zap.Logger,
	missingValues *counter,
) {
	tags := attributesToTags(numberDataPoint.Attributes())
	ts := numberDataPoint.Timestamp().AsTime().Unix()
	value, err := getValue(numberDataPoint)
	if err != nil {
		logMissingValue(metric, logger, missingValues)
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

// consumerOptions is general options for consumers
type consumerOptions struct {

	// The zap logger to use, nil means no logging
	Logger *zap.Logger

	// If true, report internal metrics to wavefront
	ReportInternalMetrics bool
}

func (c *consumerOptions) replaceZeroFieldsWithDefaults() {
	if c.Logger == nil {
		c.Logger = zap.NewNop()
	}
}

type gaugeConsumer struct {
	sender                gaugeSender
	logger                *zap.Logger
	missingValues         counter
	reportInternalMetrics bool
}

// newGaugeConsumer returns a typedMetricConsumer that consumes gauge metrics
// by sending them to tanzu observability. Caller can pass nil for options to get the defaults.
func newGaugeConsumer(sender gaugeSender, options *consumerOptions) typedMetricConsumer {
	var fixedOptions consumerOptions
	if options != nil {
		fixedOptions = *options
	}
	fixedOptions.replaceZeroFieldsWithDefaults()
	return &gaugeConsumer{
		sender:                sender,
		logger:                fixedOptions.Logger,
		reportInternalMetrics: fixedOptions.ReportInternalMetrics,
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
			g.logger,
			&g.missingValues)
	}
}

func (g *gaugeConsumer) PushInternalMetrics(errs *[]error) {
	if g.reportInternalMetrics {
		g.missingValues.Report(missingValueMetricName, typeIsGaugeTags, g.sender, errs)
	}
}

type sumConsumer struct {
	sender                senders.MetricSender
	logger                *zap.Logger
	missingValues         counter
	reportInternalMetrics bool
}

func newSumConsumer(sender senders.MetricSender, options *consumerOptions) typedMetricConsumer {
	var fixedOptions consumerOptions
	if options != nil {
		fixedOptions = *options
	}
	fixedOptions.replaceZeroFieldsWithDefaults()
	return &sumConsumer{
		sender:                sender,
		logger:                fixedOptions.Logger,
		reportInternalMetrics: fixedOptions.ReportInternalMetrics,
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
				metric, numberDataPoints.At(i), errs, s.sender, s.logger, &s.missingValues)
		}
	}
}

func (s *sumConsumer) PushInternalMetrics(errs *[]error) {
	if s.reportInternalMetrics {
		s.missingValues.Report(missingValueMetricName, typeIsSumTags, s.sender, errs)
	}
}

func (s *sumConsumer) pushNumberDataPoint(
	metric pdata.Metric, numberDataPoint pdata.NumberDataPoint, errs *[]error,
) {
	tags := attributesToTags(numberDataPoint.Attributes())
	value, err := getValue(numberDataPoint)
	if err != nil {
		logMissingValue(metric, s.logger, &s.missingValues)
		return
	}
	err = s.sender.SendDeltaCounter(metric.Name(), value, "", tags)
	if err != nil {
		*errs = append(*errs, err)
	}
}
