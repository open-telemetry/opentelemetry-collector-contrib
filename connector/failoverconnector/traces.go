// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"
	"errors"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type tracesRouter struct {
	*baseFailoverRouter[consumer.Traces]
	strategy TracesFailoverStrategy
}

func newTracesRouter(provider consumerProvider[consumer.Traces], cfg *Config) (*tracesRouter, error) {
	failover, err := newBaseFailoverRouter(provider, cfg)
	if err != nil {
		return nil, err
	}

	// Create the appropriate strategy based on the failover mode
	factory := GetFailoverStrategyFactory(cfg.FailoverMode)
	strategy := factory.CreateTracesStrategy(failover)

	return &tracesRouter{
		baseFailoverRouter: failover,
		strategy:           strategy,
	}, nil
}

// Consume is the traces-specific consumption method
func (f *tracesRouter) Consume(ctx context.Context, td ptrace.Traces) error {
	//fmt.Println("Consume called")
	return f.strategy.ConsumeTraces(ctx, td)
}

type tracesFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *tracesRouter
	logger   *zap.Logger
}

func (*tracesFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces will try to export to the current set priority level and handle failover in the case of an error
func (f *tracesFailover) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return f.failover.Consume(ctx, td)
}

func (f *tracesFailover) Shutdown(context.Context) error {
	if f.failover != nil {
		f.failover.Shutdown()
		if f.failover.strategy != nil {
			f.failover.strategy.Shutdown()
		}
	}
	return nil
}

func newTracesToTraces(set connector.Settings, cfg component.Config, traces consumer.Traces) (connector.Traces, error) {
	config := cfg.(*Config)
	tr, ok := traces.(connector.TracesRouterAndConsumer)
	if !ok {
		return nil, errors.New("consumer is not of type TracesRouter")
	}

	failover, err := newTracesRouter(tr.Consumer, config)
	if err != nil {
		return nil, err
	}

	return &tracesFailover{
		config:   config,
		failover: failover,
		logger:   set.Logger,
	}, nil
}
