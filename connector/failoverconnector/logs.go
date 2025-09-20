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
	strategy LogsFailoverStrategy
}

func newLogsRouter(provider consumerProvider[consumer.Logs], cfg *Config) (*logsRouter, error) {
	failover, err := newBaseFailoverRouter(provider, cfg)
	if err != nil {
		return nil, err
	}
	
	// Create the appropriate strategy based on the failover mode
	factory := GetFailoverStrategyFactory(cfg.FailoverMode)
	strategy := factory.CreateLogsStrategy(failover)
	
	return &logsRouter{
		baseFailoverRouter: failover,
		strategy:          strategy,
	}, nil
}

// Consume is the logs-specific consumption method
func (f *logsRouter) Consume(ctx context.Context, ld plog.Logs) error {
	return f.strategy.ConsumeLogs(ctx, ld)
}

type logsFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *logsRouter
	logger   *zap.Logger
}

func (*logsFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs will try to export to the current set priority level and handle failover in the case of an error
func (f *logsFailover) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return f.failover.Consume(ctx, ld)
}

func (f *logsFailover) Shutdown(context.Context) error {
	if f.failover != nil {
		f.failover.Shutdown()
		if f.failover.strategy != nil {
			f.failover.strategy.Shutdown()
		}
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
