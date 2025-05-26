// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"
import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logsRouter struct {
	*baseFailoverRouter[consumer.Logs]
}

func newLogsRouter(provider consumerProvider[consumer.Logs], cfg *Config) (*logsRouter, error) {
	failover, err := newBaseFailoverRouter(provider, cfg)
	if err != nil {
		return nil, err
	}
	return &logsRouter{baseFailoverRouter: failover}, nil
}

// Consume is the logs-specific consumption method
func (f *logsRouter) Consume(ctx context.Context, ld plog.Logs) error {
	select {
	case <-f.notifyRetry:
		if !f.sampleRetryConsumers(ctx, ld) {
			return f.consumeByHealthyPipeline(ctx, ld)
		}
		return nil
	default:
		return f.consumeByHealthyPipeline(ctx, ld)
	}
}

// consumeByHealthyPipeline will consume the logs by the current healthy level
func (f *logsRouter) consumeByHealthyPipeline(ctx context.Context, ld plog.Logs) error {
	for {
		tc, idx := f.getCurrentConsumer()
		if idx >= len(f.cfg.PipelinePriority) {
			return errNoValidPipeline
		}

		if err := tc.ConsumeLogs(ctx, ld); err != nil {
			f.reportConsumerError(idx)
			continue
		}

		return nil
	}
}

// sampleRetryConsumers iterates through all unhealthy consumers to re-establish a healthy connection
func (f *logsRouter) sampleRetryConsumers(ctx context.Context, ld plog.Logs) bool {
	stableIndex := f.pS.CurrentPipeline()
	for i := 0; i < stableIndex; i++ {
		consumer := f.getConsumerAtIndex(i)
		err := consumer.ConsumeLogs(ctx, ld)
		if err == nil {
			f.pS.ResetHealthyPipeline(i)
			return true
		}
	}
	return false
}

type logsFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *logsRouter
	logger   *zap.Logger
}

func (f *logsFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs will try to export to the current set priority level and handle failover in the case of an error
func (f *logsFailover) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return f.failover.Consume(ctx, ld)
}

func (f *logsFailover) Shutdown(_ context.Context) error {
	if f.failover != nil {
		f.failover.Shutdown()
	}
	return nil
}

func newLogsToLogs(set connector.Settings, cfg component.Config, logs consumer.Logs) (connector.Logs, error) {
	config := cfg.(*Config)
	lr, ok := logs.(connector.LogsRouterAndConsumer)
	if !ok {
		return nil, errors.New("consumer is not of type MetricsRouter")
	}

	failover, err := newLogsRouter(lr.Consumer, config)
	if err != nil {
		return nil, err
	}

	return &logsFailover{
		config:   config,
		failover: failover,
		logger:   set.Logger,
	}, nil
}
