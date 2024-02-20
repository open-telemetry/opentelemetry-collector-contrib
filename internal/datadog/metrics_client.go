// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadog // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog"

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type metricsClient struct {
	meter  metric.Meter
	gauges map[string]float64
	mutex  sync.Mutex
}

var initializeOnce sync.Once

// InitializeMetricClient using a meter provider. If the client has already been initialized,
// this function just returns the previous one.
func InitializeMetricClient(mp metric.MeterProvider) metrics.StatsClient {
	initializeOnce.Do(func() {
		m := &metricsClient{
			meter:  mp.Meter("datadog"),
			gauges: make(map[string]float64),
		}
		metrics.Client = m
	})
	return metrics.Client
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
		attr := attributeFromTags(tags)
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
	attr := attributeFromTags(tags)
	counter.Add(context.Background(), value, metric.WithAttributeSet(attr))
	return nil
}

func attributeFromTags(tags []string) attribute.Set {
	attr := make([]attribute.KeyValue, 0, len(tags))
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
	attr := attributeFromTags(tags)
	hist.Record(context.Background(), value, metric.WithAttributeSet(attr))
	return nil
}

func (m *metricsClient) Timing(name string, value time.Duration, tags []string, rate float64) error {
	return m.Histogram(name, float64(value.Milliseconds()), tags, rate)
}

func (m *metricsClient) Flush() error {
	return nil
}
