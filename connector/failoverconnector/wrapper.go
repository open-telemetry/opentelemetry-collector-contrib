// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type WrappedTracesConnector struct {
	consumer     consumer.Traces
	failoverCore *tracesFailover
}

type WrappedMetricsConnector struct {
	consumer     consumer.Metrics
	failoverCore *metricsFailover
}

type WrappedLogsConnector struct {
	consumer     consumer.Logs
	failoverCore *logsFailover
}

func (w *WrappedTracesConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return w.consumer.ConsumeTraces(ctx, td)
}

func (w *WrappedTracesConnector) Capabilities() consumer.Capabilities {
	return w.consumer.Capabilities()
}

func (w *WrappedTracesConnector) Start(ctx context.Context, host component.Host) error {
	if starter, ok := w.consumer.(component.Component); ok {
		return starter.Start(ctx, host)
	}
	return nil
}

func (w *WrappedMetricsConnector) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return w.consumer.ConsumeMetrics(ctx, md)
}

func (w *WrappedMetricsConnector) Capabilities() consumer.Capabilities {
	return w.consumer.Capabilities()
}

func (w *WrappedMetricsConnector) Start(ctx context.Context, host component.Host) error {
	if starter, ok := w.consumer.(component.Component); ok {
		return starter.Start(ctx, host)
	}
	return nil
}

func (w *WrappedLogsConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return w.consumer.ConsumeLogs(ctx, ld)
}

func (w *WrappedLogsConnector) Capabilities() consumer.Capabilities {
	return w.consumer.Capabilities()
}

func (w *WrappedLogsConnector) Start(ctx context.Context, host component.Host) error {
	if starter, ok := w.consumer.(component.Component); ok {
		return starter.Start(ctx, host)
	}
	return nil
}

func (w *WrappedTracesConnector) GetFailoverRouter() *tracesRouter {
	return w.failoverCore.failover
}

func (w *WrappedMetricsConnector) GetFailoverRouter() *metricsRouter {
	return w.failoverCore.failover
}

func (w *WrappedLogsConnector) GetFailoverRouter() *logsRouter {
	return w.failoverCore.failover
}

func (w *WrappedTracesConnector) Shutdown(ctx context.Context) error {
	var err error
	if shutdowner, ok := w.consumer.(interface{ Shutdown(context.Context) error }); ok {
		err = shutdowner.Shutdown(ctx)
	}
	w.failoverCore.failover.Shutdown()
	return err
}

func (w *WrappedMetricsConnector) Shutdown(ctx context.Context) error {
	var err error
	if shutdowner, ok := w.consumer.(interface{ Shutdown(context.Context) error }); ok {
		err = shutdowner.Shutdown(ctx)
	}
	w.failoverCore.failover.Shutdown()
	return err
}

func (w *WrappedLogsConnector) Shutdown(ctx context.Context) error {
	var err error
	if shutdowner, ok := w.consumer.(interface{ Shutdown(context.Context) error }); ok {
		err = shutdowner.Shutdown(ctx)
	}
	w.failoverCore.failover.Shutdown()
	return err
}

func NewWrappedTracesConnector(consumer consumer.Traces, failoverCore *tracesFailover) *WrappedTracesConnector {
	return &WrappedTracesConnector{
		consumer:     consumer,
		failoverCore: failoverCore,
	}
}

func NewWrappedMetricsConnector(consumer consumer.Metrics, failoverCore *metricsFailover) *WrappedMetricsConnector {
	return &WrappedMetricsConnector{
		consumer:     consumer,
		failoverCore: failoverCore,
	}
}

func NewWrappedLogsConnector(consumer consumer.Logs, failoverCore *logsFailover) *WrappedLogsConnector {
	return &WrappedLogsConnector{
		consumer:     consumer,
		failoverCore: failoverCore,
	}
}
