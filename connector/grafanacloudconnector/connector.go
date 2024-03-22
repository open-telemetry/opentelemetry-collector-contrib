// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grafanacloudconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/grafanacloudconnector"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/grafanacloudconnector/internal/metadata"
)

const (
	hostInfoMetric     = "traces_host_info"
	hostIdentifierAttr = "grafana.host.id"
)

var _ connector.Traces = (*connectorImp)(nil)

type connectorImp struct {
	config Config
	logger *zap.Logger

	started      bool
	done         chan struct{}
	shutdownOnce sync.Once

	metricsConsumer consumer.Metrics
	hostMetrics     *hostMetrics

	metricHostCount      metric.Int64ObservableGauge
	metricFlushCount     metric.Int64Counter
	metricDatapointCount metric.Int64Counter
}

func newConnector(logger *zap.Logger, set component.TelemetrySettings, config component.Config) (*connectorImp, error) {
	hm := newHostMetrics()
	mHostCount, err := metadata.Meter(set).Int64ObservableGauge(
		"grafanacloud_host_count",
		metric.WithDescription("Number of unique hosts"),
		metric.WithUnit("1"),
		metric.WithInt64Callback(func(ctx context.Context, result metric.Int64Observer) error {
			result.Observe(int64(hm.count()))
			return nil
		}),
	)

	if err != nil {
		return nil, err
	}

	mFlushCount, err := metadata.Meter(set).Int64Counter(
		"grafanacloud_flush_count",
		metric.WithDescription("Number of metrics flushes"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	mDatapointCount, err := metadata.Meter(set).Int64Counter(
		"grafanacloud_datapoint_count",
		metric.WithDescription("Number of datapoints sent to Grafana Cloud"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}

	cfg := config.(*Config)
	return &connectorImp{
		config:               *cfg,
		logger:               logger,
		done:                 make(chan struct{}),
		hostMetrics:          hm,
		metricHostCount:      mHostCount,
		metricFlushCount:     mFlushCount,
		metricDatapointCount: mDatapointCount,
	}, nil
}

// Capabilities implements connector.Traces.
func (c *connectorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements connector.Traces.
func (c *connectorImp) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resourceSpan := td.ResourceSpans().At(i)
		attrs := resourceSpan.Resource().Attributes()
		mapping := attrs.AsRaw()

		for _, attrName := range c.config.HostIdentifiers {
			if val, ok := mapping[attrName]; ok {
				if v, ok := val.(string); ok {
					c.hostMetrics.add(v)
				}
				break
			}
		}
	}
	return nil
}

// Start implements connector.Traces.
func (c *connectorImp) Start(ctx context.Context, _ component.Host) error {
	c.logger.Info("Starting Grafana Cloud connector")
	c.started = true
	ticker := time.NewTicker(c.config.MetricsFlushInterval)
	go func() {
		for {
			select {
			case <-c.done:
				ticker.Stop()
				return
			case <-ticker.C:
				if err := c.flush(ctx); err != nil {
					c.logger.Error("Error consuming metrics", zap.Error(err))
				}
			}
		}
	}()
	return nil
}

// Shutdown implements connector.Traces.
func (c *connectorImp) Shutdown(ctx context.Context) error {
	c.shutdownOnce.Do(func() {
		c.logger.Info("Stopping Grafana Cloud connector")
		if c.started {
			// flush metrics on shutdown
			if err := c.flush(ctx); err != nil {
				c.logger.Error("Error consuming metrics", zap.Error(err))
			}
			c.done <- struct{}{}
			c.started = false
		}
	})
	return nil
}

func (c *connectorImp) flush(ctx context.Context) error {
	var err error

	metrics, count := c.hostMetrics.metrics()
	if count > 0 {
		c.hostMetrics.reset()
		c.logger.Debug("Flushing metrics", zap.Int("count", count))
		c.metricDatapointCount.Add(ctx, int64(metrics.DataPointCount()))
		err = c.metricsConsumer.ConsumeMetrics(ctx, *metrics)
	}
	c.metricFlushCount.Add(ctx, int64(1))
	return err
}
