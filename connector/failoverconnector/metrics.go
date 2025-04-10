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
}

func newMetricsRouter(provider consumerProvider[consumer.Metrics], cfg *Config) (*metricsRouter, error) {
	failover, err := newBaseFailoverRouter(provider, cfg)
	if err != nil {
		return nil, err
	}
	return &metricsRouter{baseFailoverRouter: failover}, nil
}

// Consume is the metrics-specific consumption method
func (f *metricsRouter) Consume(ctx context.Context, md pmetric.Metrics) error {
	select {
	case <-f.notifyRetry:
		if !f.sampleRetryConsumers(ctx, md) {
			return f.consumeByHealthyPipeline(ctx, md)
		}
		return nil
	default:
		return f.consumeByHealthyPipeline(ctx, md)
	}
}

// consumeByHealthyPipeline will consume the metrics by the current healthy level
func (f *metricsRouter) consumeByHealthyPipeline(ctx context.Context, md pmetric.Metrics) error {
	for {
		tc, idx := f.getCurrentConsumer()
		if idx >= len(f.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		if err := tc.ConsumeMetrics(ctx, md); err != nil {
			f.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

// sampleRetryConsumers iterates through all unhealthy consumers to re-establish a healthy connection
func (f *metricsRouter) sampleRetryConsumers(ctx context.Context, md pmetric.Metrics) bool {
	stableIndex := f.pS.CurrentPipeline()
	for i := 0; i < stableIndex; i++ {
		consumer := f.getConsumerAtIndex(i)
		err := consumer.ConsumeMetrics(ctx, md)
		if err == nil {
			f.pS.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

type metricsFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *metricsRouter
	logger   *zap.Logger
}

func (f *metricsFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeMetrics will try to export to the current set priority level and handle failover in the case of an error
func (f *metricsFailover) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return f.failover.Consume(ctx, md)
}

func (f *metricsFailover) Shutdown(_ context.Context) error {
	if f.failover != nil {
		f.failover.Shutdown()
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
