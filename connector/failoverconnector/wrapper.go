// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package failoverconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type wrappedTracesConnector struct {
	consumer     consumer.Traces
	failoverCore *tracesFailover
}

type wrappedMetricsConnector struct {
	consumer     consumer.Metrics
	failoverCore *metricsFailover
}

type wrappedLogsConnector struct {
	consumer     consumer.Logs
	failoverCore *logsFailover
}

func (w *wrappedTracesConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return w.consumer.ConsumeTraces(ctx, td)
}

func (w *wrappedTracesConnector) Capabilities() consumer.Capabilities {
	return w.consumer.Capabilities()
}

func (w *wrappedTracesConnector) Start(ctx context.Context, host component.Host) error {
	if starter, ok := w.consumer.(component.Component); ok {
		return starter.Start(ctx, host)
	}
	return nil
}

func (w *wrappedMetricsConnector) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return w.consumer.ConsumeMetrics(ctx, md)
}

func (w *wrappedMetricsConnector) Capabilities() consumer.Capabilities {
	return w.consumer.Capabilities()
}

func (w *wrappedMetricsConnector) Start(ctx context.Context, host component.Host) error {
	if starter, ok := w.consumer.(component.Component); ok {
		return starter.Start(ctx, host)
	}
	return nil
}

func (w *wrappedLogsConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return w.consumer.ConsumeLogs(ctx, ld)
}

func (w *wrappedLogsConnector) Capabilities() consumer.Capabilities {
	return w.consumer.Capabilities()
}

func (w *wrappedLogsConnector) Start(ctx context.Context, host component.Host) error {
	if starter, ok := w.consumer.(component.Component); ok {
		return starter.Start(ctx, host)
	}
	return nil
}

func (w *wrappedTracesConnector) GetFailoverRouter() *tracesRouter {
	return w.failoverCore.failover
}

func (w *wrappedMetricsConnector) GetFailoverRouter() *metricsRouter {
	return w.failoverCore.failover
}

func (w *wrappedLogsConnector) GetFailoverRouter() *logsRouter {
	return w.failoverCore.failover
}

func (w *wrappedTracesConnector) Shutdown(ctx context.Context) error {
	var err error
	if shutdowner, ok := w.consumer.(interface{ Shutdown(context.Context) error }); ok {
		err = shutdowner.Shutdown(ctx)
	}
	w.failoverCore.failover.Shutdown()
	return err
}

func (w *wrappedMetricsConnector) Shutdown(ctx context.Context) error {
	var err error
	if shutdowner, ok := w.consumer.(interface{ Shutdown(context.Context) error }); ok {
		err = shutdowner.Shutdown(ctx)
	}
	w.failoverCore.failover.Shutdown()
	return err
}

func (w *wrappedLogsConnector) Shutdown(ctx context.Context) error {
	var err error
	if shutdowner, ok := w.consumer.(interface{ Shutdown(context.Context) error }); ok {
		err = shutdowner.Shutdown(ctx)
	}
	w.failoverCore.failover.Shutdown()
	return err
}

func newWrappedTracesConnector(consumer consumer.Traces, failoverCore *tracesFailover) *wrappedTracesConnector {
	return &wrappedTracesConnector{
		consumer:     consumer,
		failoverCore: failoverCore,
	}
}

func newWrappedMetricsConnector(consumer consumer.Metrics, failoverCore *metricsFailover) *wrappedMetricsConnector {
	return &wrappedMetricsConnector{
		consumer:     consumer,
		failoverCore: failoverCore,
	}
}

func newWrappedLogsConnector(consumer consumer.Logs, failoverCore *logsFailover) *wrappedLogsConnector {
	return &wrappedLogsConnector{
		consumer:     consumer,
		failoverCore: failoverCore,
	}
}
