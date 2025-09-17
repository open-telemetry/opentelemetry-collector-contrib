// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"
import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricsRouter struct {
	*baseFailoverRouter[consumer.Metrics]
	strategy MetricsFailoverStrategy
}

func newMetricsRouter(provider consumerProvider[consumer.Metrics], cfg *Config) (*metricsRouter, error) {
	failover, err := newBaseFailoverRouter(provider, cfg)
	if err != nil {
		return nil, err
	}
	
	// Create the appropriate strategy based on the failover mode
	factory := GetFailoverStrategyFactory(cfg.FailoverMode)
	strategy := factory.CreateMetricsStrategy(failover)
	
	return &metricsRouter{
		baseFailoverRouter: failover,
		strategy:          strategy,
	}, nil
}

// Consume is the metrics-specific consumption method
func (f *metricsRouter) Consume(ctx context.Context, md pmetric.Metrics) error {
	return f.strategy.ConsumeMetrics(ctx, md)
}

type metricsFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *metricsRouter
	logger   *zap.Logger
}

func (*metricsFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeMetrics will try to export to the current set priority level and handle failover in the case of an error
func (f *metricsFailover) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return f.failover.Consume(ctx, md)
}

func (f *metricsFailover) Shutdown(context.Context) error {
	if f.failover != nil {
		f.failover.Shutdown()
		if f.failover.strategy != nil {
			f.failover.strategy.Shutdown()
		}
	}
	return nil
}

func newMetricsToMetrics(set connector.Settings, cfg component.Config, metrics consumer.Metrics) (connector.Metrics, error) {
	config := cfg.(*Config)
	mr, ok := metrics.(connector.MetricsRouterAndConsumer)
	if !ok {
		return nil, errors.New("consumer is not of type MetricsRouter")
	}

	failover, err := newMetricsRouter(mr.Consumer, config)
	if err != nil {
		return nil, err
	}

	return &metricsFailover{
		config:   config,
		failover: failover,
		logger:   set.Logger,
	}, nil
}
