// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector/internal"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var errConsumeType = errors.New("error in consume type assertion")

var (
	_ SignalConsumer = (*TracesWrapper)(nil)
	_ SignalConsumer = (*MetricsWrapper)(nil)
	_ SignalConsumer = (*LogsWrapper)(nil)
)

type SignalConsumer interface {
	Consume(context.Context, any) error
}

func NewTracesWrapper(c consumer.Traces) *TracesWrapper {
	return &TracesWrapper{c}
}

type TracesWrapper struct {
	consumer.Traces
}

func (t *TracesWrapper) Consume(ctx context.Context, pd any) error {
	td, ok := pd.(ptrace.Traces)
	if !ok {
		return errConsumeType
	}
	return t.ConsumeTraces(ctx, td)
}

func NewMetricsWrapper(c consumer.Metrics) *MetricsWrapper {
	return &MetricsWrapper{c}
}

type MetricsWrapper struct {
	consumer.Metrics
}

func (t *MetricsWrapper) Consume(ctx context.Context, pd any) error {
	td, ok := pd.(pmetric.Metrics)
	if !ok {
		return errConsumeType
	}
	return t.ConsumeMetrics(ctx, td)
}

func NewLogsWrapper(c consumer.Logs) *LogsWrapper {
	return &LogsWrapper{c}
}

type LogsWrapper struct {
	consumer.Logs
}

func (t *LogsWrapper) Consume(ctx context.Context, pd any) error {
	td, ok := pd.(plog.Logs)
	if !ok {
		return errConsumeType
	}
	return t.ConsumeLogs(ctx, td)
}
