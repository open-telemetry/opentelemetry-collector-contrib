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

type tracesFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config   *Config
	failover *failoverRouter[ptrace.Traces, consumer.Traces]
	logger   *zap.Logger
}

func (f *tracesFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (f *tracesFailover) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return f.failover.Consume(ctx, td)
}

func (f *tracesFailover) Shutdown(_ context.Context) error {
	if f.failover != nil {
		f.failover.Shutdown()
	}
	return nil
}

func newTracesToTraces(set connector.Settings, cfg component.Config, traces consumer.Traces) (connector.Traces, error) {
	config := cfg.(*Config)
	tr, ok := traces.(connector.TracesRouterAndConsumer)
	if !ok {
		return nil, errors.New("consumer is not of type TracesRouter")
	}

	failover := newFailoverRouter[ptrace.Traces, consumer.Traces](tr.Consumer, config)
	err := failover.registerConsumers()
	if err != nil {
		return nil, err
	}

	return &tracesFailover{
		config:   config,
		failover: failover,
		logger:   set.TelemetrySettings.Logger,
	}, nil
}
