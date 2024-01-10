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

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"
)

type logsFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config        *Config
	failover      *failoverRouter[consumer.Logs]
	logger        *zap.Logger
	errTryLock    *state.TryLock
	stableTryLock *state.TryLock
}

func (f *logsFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs will try to export to the current set priority level and handle failover in the case of an error
func (f *logsFailover) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	mc, idx, ok := f.failover.getCurrentConsumer()
	if !ok {
		return errNoValidPipeline
	}
	err := mc.ConsumeLogs(ctx, ld)
	if err == nil {
		f.stableTryLock.TryExecute(f.failover.reportStable, idx)
		return nil
	}
	return f.FailoverLogs(ctx, ld)
}

// FailoverLogs is the function responsible for handling errors returned by the nextConsumer
func (f *logsFailover) FailoverLogs(ctx context.Context, ld plog.Logs) error {
	for mc, idx, ok := f.failover.getCurrentConsumer(); ok; mc, idx, ok = f.failover.getCurrentConsumer() {
		err := mc.ConsumeLogs(ctx, ld)
		if err != nil {
			f.errTryLock.TryExecute(f.failover.handlePipelineError, idx)
			continue
		}
		f.stableTryLock.TryExecute(f.failover.reportStable, idx)
		return nil
	}
	f.logger.Error("All provided pipelines return errors, dropping data")
	return errNoValidPipeline
}

func (f *logsFailover) Shutdown(_ context.Context) error {
	if f.failover != nil {
		f.failover.rS.InvokeCancel()
	}
	return nil
}

func newLogsToLogs(set connector.CreateSettings, cfg component.Config, logs consumer.Logs) (connector.Logs, error) {
	config := cfg.(*Config)
	lr, ok := logs.(connector.LogsRouterAndConsumer)
	if !ok {
		return nil, errors.New("consumer is not of type LogsRouter")
	}

	failover := newFailoverRouter[consumer.Logs](lr.Consumer, config) // temp add type spec to resolve linter issues
	err := failover.registerConsumers()
	if err != nil {
		return nil, err
	}
	return &logsFailover{
		config:        config,
		failover:      failover,
		logger:        set.TelemetrySettings.Logger,
		errTryLock:    state.NewTryLock(),
		stableTryLock: state.NewTryLock(),
	}, nil
}
