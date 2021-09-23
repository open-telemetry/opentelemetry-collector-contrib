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

package tanzuobservabilityexporter

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
)

// metricsConsumer instances consume OTEL metrics
type metricsConsumer struct {
	consumerMap map[pdata.MetricDataType]metricConsumer
	sender      flushCloser
}

// newMetricsConsumer returns a new metricsConsumer. consumers are the
// consumers responsible for consuming each type metric. The Consume method
// of returned consumer calls the Flush method on sender after consuming
// all the metrics. Calling Close on the returned metricsConsumer calls Close
// on sender. sender can be nil.
func newMetricsConsumer(
	consumers []metricConsumer, sender flushCloser) *metricsConsumer {
	consumerMap := make(map[pdata.MetricDataType]metricConsumer, len(consumers))
	for _, consumer := range consumers {
		consumerMap[consumer.Type()] = consumer
	}
	return &metricsConsumer{
		consumerMap: consumerMap,
		sender:      sender,
	}
}

// Consume consumes OTEL metrics. For each metric in md, it delegates to the
// metricConsumer that consumes that type of metric. Once Consume consumes
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
					return consumererror.Combine(append(errs, errors.New("context canceled")))
				default:
					c.pushSingleMetric(m, &errs)
				}
			}
		}
	}
	if c.sender != nil {
		if err := c.sender.Flush(); err != nil {
			errs = append(errs, err)
		}
	}
	return consumererror.Combine(errs)
}

// Close closes this metricsConsumer by calling Close on the sender passed
// to newMetricsConsumer.
func (c *metricsConsumer) Close() {
	if c.sender != nil {
		c.sender.Close()
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

// metricConsumer consumes one specific type of OTEL metric
type metricConsumer interface {

	// Type returns the type of metric this consumer consumes. For example
	// Gauge, Sum, or Histogram
	Type() pdata.MetricDataType

	// Consume consumes the metric and appends any errors encountered to errs
	Consume(m pdata.Metric, errs *[]error)
}

// flushCloser is the interface for the Flush and Close method
type flushCloser interface {
	Flush() error
	Close()
}

// gaugeSender sends gauge metrics to tanzu observability
type gaugeSender interface {
	SendMetric(name string, value float64, ts int64, source string, tags map[string]string) error
}

type gaugeConsumer struct {
	sender gaugeSender
}

// newGaugeConsumer returns a metricConsumer that consumes gauge metrics
// by sending them to tanzu observability
func newGaugeConsumer(sender gaugeSender) metricConsumer {
	return &gaugeConsumer{sender: sender}
}

func (g *gaugeConsumer) Type() pdata.MetricDataType {
	return pdata.MetricDataTypeGauge
}

func (g *gaugeConsumer) Consume(metric pdata.Metric, errs *[]error) {
	gauge := metric.Gauge()
	numberDataPoints := gauge.DataPoints()
	for i := 0; i < numberDataPoints.Len(); i++ {
		g.pushSingleNumberDataPoint(metric, numberDataPoints.At(i), errs)
	}
}

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

func (g *gaugeConsumer) pushSingleNumberDataPoint(
	metric pdata.Metric, numberDataPoint pdata.NumberDataPoint, errs *[]error) {
	tags := attributesToTags(numberDataPoint.Attributes())
	ts := numberDataPoint.Timestamp().AsTime().Unix()
	value, err := getValue(numberDataPoint)
	if err != nil {
		*errs = append(*errs, err)
		return
	}
	err = g.sender.SendMetric(metric.Name(), value, ts, "", tags)
	if err != nil {
		*errs = append(*errs, err)
	}
}
