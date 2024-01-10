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

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"
)

type tracesFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config        *Config
	failover      *failoverRouter[consumer.Traces]
	logger        *zap.Logger
	errTryLock    *state.TryLock
	stableTryLock *state.TryLock
}

func (f *tracesFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces will try to export to the current set priority level and handle failover in the case of an error
func (f *tracesFailover) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	tc, idx, ok := f.failover.getCurrentConsumer()
	if !ok {
		return errNoValidPipeline
	}
	err := tc.ConsumeTraces(ctx, td)
	if err == nil {
		// trylock to make sure for concurrent calls multiple invocations don't try to update the failover
		// state simultaneously and don't wait to acquire lock in pipeline selector
		f.stableTryLock.TryExecute(f.failover.reportStable, idx)
		return nil
	}
	return f.FailoverTraces(ctx, td)
}

// FailoverTraces is the function responsible for handling errors returned by the nextConsumer
func (f *tracesFailover) FailoverTraces(ctx context.Context, td ptrace.Traces) error {
	// loops through consumer list, until reaches end of list at which point no healthy pipelines remain
	for tc, idx, ok := f.failover.getCurrentConsumer(); ok; tc, idx, ok = f.failover.getCurrentConsumer() {
		err := tc.ConsumeTraces(ctx, td)
		if err != nil {
			// in case of err handlePipelineError is called through tryLock
			// tryLock is to avoid race conditions from concurrent calls to handlePipelineError, only first call should enter
			// see state.TryLock
			f.errTryLock.TryExecute(f.failover.handlePipelineError, idx)
			continue
		}
		// when healthy pipeline is found, reported back to failover component
		f.stableTryLock.TryExecute(f.failover.reportStable, idx)
		return nil
	}
	f.logger.Error("All provided pipelines return errors, dropping data")
	return errNoValidPipeline
}

func (f *tracesFailover) Shutdown(_ context.Context) error {
	// call cancel mainly to shutdown tickers
	if f.failover != nil {
		f.failover.rS.InvokeCancel()
	}
	return nil
}

func newTracesToTraces(set connector.CreateSettings, cfg component.Config, traces consumer.Traces) (connector.Traces, error) {
	config := cfg.(*Config)
	tr, ok := traces.(connector.TracesRouterAndConsumer)
	if !ok {
		return nil, errors.New("consumer is not of type TracesRouter")
	}

	failover := newFailoverRouter[consumer.Traces](tr.Consumer, config) // temp add type spec to resolve linter issues
	err := failover.registerConsumers()
	if err != nil {
		return nil, err
	}
	return &tracesFailover{
		config:        config,
		failover:      failover,
		logger:        set.TelemetrySettings.Logger,
		errTryLock:    state.NewTryLock(),
		stableTryLock: state.NewTryLock(),
	}, nil
}
