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

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal/state"
)

type metricsFailover struct {
	component.StartFunc
	component.ShutdownFunc

	config        *Config
	failover      *failoverRouter[consumer.Metrics]
	logger        *zap.Logger
	errTryLock    *state.TryLock
	stableTryLock *state.TryLock
}

func (f *metricsFailover) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeMetrics will try to export to the current set priority level and handle failover in the case of an error
func (f *metricsFailover) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	mc, idx, ok := f.failover.getCurrentConsumer()
	if !ok {
		return errNoValidPipeline
	}
	err := mc.ConsumeMetrics(ctx, md)
	if err == nil {
		f.stableTryLock.TryExecute(f.failover.reportStable, idx)
		return nil
	}
	return f.FailoverMetrics(ctx, md)
}

// FailoverMetrics is the function responsible for handling errors returned by the nextConsumer
func (f *metricsFailover) FailoverMetrics(ctx context.Context, md pmetric.Metrics) error {
	for mc, idx, ok := f.failover.getCurrentConsumer(); ok; mc, idx, ok = f.failover.getCurrentConsumer() {
		err := mc.ConsumeMetrics(ctx, md)
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

func (f *metricsFailover) Shutdown(_ context.Context) error {
	if f.failover != nil {
		f.failover.rS.InvokeCancel()
	}
	return nil
}

func newMetricsToMetrics(set connector.CreateSettings, cfg component.Config, metrics consumer.Metrics) (connector.Metrics, error) {
	config := cfg.(*Config)
	mr, ok := metrics.(connector.MetricsRouterAndConsumer)
	if !ok {
		return nil, errors.New("consumer is not of type MetricsRouter")
	}

	failover := newFailoverRouter[consumer.Metrics](mr.Consumer, config)
	err := failover.registerConsumers()
	if err != nil {
		return nil, err
	}
	return &metricsFailover{
		config:        config,
		failover:      failover,
		logger:        set.TelemetrySettings.Logger,
		errTryLock:    state.NewTryLock(),
		stableTryLock: state.NewTryLock(),
	}, nil
}
