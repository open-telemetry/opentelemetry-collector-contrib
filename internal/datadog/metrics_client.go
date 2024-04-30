// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadog // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog"

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	ExporterSourceTag  = "datadogexporter"
	ConnectorSourceTag = "datadogconnector"
)

type metricsClient struct {
	meter  metric.Meter
	gauges map[string]float64
	mutex  sync.Mutex
	source string
}

// InitializeMetricClient using a meter provider.
func InitializeMetricClient(mp metric.MeterProvider, source string) statsd.ClientInterface {
	return &metricsClient{
		meter:  mp.Meter("datadog"),
		gauges: make(map[string]float64),
		source: source,
	}
}

func (m *metricsClient) Gauge(name string, value float64, tags []string, _ float64) error {
	// The last parameter is rate, but we're omitting it because rate does not have effect for gauge points: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/dedd44436ae064f5a0b43769d24adf897533957b/receiver/statsdreceiver/internal/protocol/metric_translator.go#L153-L156
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, ok := m.gauges[name]; ok {
		m.gauges[name] = value
		return nil
	}
	m.gauges[name] = value
	_, err := m.meter.Float64ObservableGauge(name, metric.WithFloat64Callback(func(_ context.Context, f metric.Float64Observer) error {
		attr := m.attributeFromTags(tags)
		m.mutex.Lock()
		defer m.mutex.Unlock()
		if v, ok := m.gauges[name]; ok {
			f.Observe(v, metric.WithAttributeSet(attr))
		}
		return nil
	}))
	if err != nil {
		return err
	}
	return nil
}

func (m *metricsClient) Count(name string, value int64, tags []string, _ float64) error {
	counter, err := m.meter.Int64Counter(name)
	if err != nil {
		return err
	}
	attr := m.attributeFromTags(tags)
	counter.Add(context.Background(), value, metric.WithAttributeSet(attr))
	return nil
}

func (m *metricsClient) attributeFromTags(tags []string) attribute.Set {
	attr := make([]attribute.KeyValue, 0, len(tags)+1)
	attr = append(attr, attribute.KeyValue{
		Key:   "source",
		Value: attribute.StringValue(m.source),
	})
	for _, t := range tags {
		kv := strings.Split(t, ":")
		attr = append(attr, attribute.KeyValue{
			Key:   attribute.Key(kv[0]),
			Value: attribute.StringValue(kv[1]),
		})
	}
	return attribute.NewSet(attr...)
}

func (m *metricsClient) Histogram(name string, value float64, tags []string, _ float64) error {
	hist, err := m.meter.Float64Histogram(name)
	if err != nil {
		return err
	}
	attr := m.attributeFromTags(tags)
	hist.Record(context.Background(), value, metric.WithAttributeSet(attr))
	return nil
}

func (m *metricsClient) Distribution(name string, value float64, tags []string, rate float64) error {
	return m.Histogram(name, value, tags, rate)
}

func (m *metricsClient) Timing(name string, value time.Duration, tags []string, rate float64) error {
	return m.TimeInMilliseconds(name, value.Seconds()*1000, tags, rate)
}

func (m *metricsClient) TimeInMilliseconds(name string, value float64, tags []string, rate float64) error {
	return m.Histogram(name, value, tags, rate)
}

func (m *metricsClient) Decr(name string, tags []string, rate float64) error {
	return m.Count(name, -1, tags, rate)
}

func (m *metricsClient) Incr(name string, tags []string, rate float64) error {
	return m.Count(name, 1, tags, rate)
}

func (m *metricsClient) Flush() error {
	return nil
}

func (m *metricsClient) Set(string, string, []string, float64) error {
	return nil
}

func (m *metricsClient) Event(*statsd.Event) error {
	return nil
}

func (m *metricsClient) SimpleEvent(string, string) error {
	return nil
}

func (m *metricsClient) ServiceCheck(*statsd.ServiceCheck) error {
	return nil
}

func (m *metricsClient) SimpleServiceCheck(string, statsd.ServiceCheckStatus) error {
	return nil
}

func (m *metricsClient) Close() error {
	return nil
}

func (m *metricsClient) IsClosed() bool {
	return false
}

func (m *metricsClient) GetTelemetry() statsd.Telemetry {
	return statsd.Telemetry{}
}

func (m *metricsClient) GaugeWithTimestamp(string, float64, []string, float64, time.Time) error {
	return nil
}

func (m *metricsClient) CountWithTimestamp(string, int64, []string, float64, time.Time) error {
	return nil
}
