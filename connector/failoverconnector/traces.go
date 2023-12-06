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
	failover *failoverRouter[consumer.Traces]
	logger   *zap.Logger
}

func (f *tracesFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces will try to export to the current set priority level and handle failover in the case of an error
func (f *tracesFailover) ConsumeTraces(_ context.Context, _ ptrace.Traces) error {
	return nil
}

func (f *tracesFailover) Shutdown(_ context.Context) error {
	return nil
}

func newTracesToTraces(set connector.CreateSettings, cfg component.Config, traces consumer.Traces) (connector.Traces, error) {
	config := cfg.(*Config)
	tr, ok := traces.(connector.TracesRouter)
	if !ok {
		return nil, errors.New("consumer is not of type TracesRouter")
	}

	failover := newFailoverRouter[consumer.Traces](tr.Consumer, config) // temp add type spec to resolve linter issues
	return &tracesFailover{
		config:   config,
		failover: failover,
		logger:   set.TelemetrySettings.Logger,
	}, nil
}
