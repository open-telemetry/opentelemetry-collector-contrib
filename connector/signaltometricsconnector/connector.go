// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signaltometricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type signalToMetrics struct {
	next   consumer.Metrics
	logger *zap.Logger

	component.StartFunc
	component.ShutdownFunc
}

func newSignalToMetrics(
	set connector.Settings,
	next consumer.Metrics,
) *signalToMetrics {
	return &signalToMetrics{
		logger: set.Logger,
		next:   next,
	}
}

func (sm *signalToMetrics) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (sm *signalToMetrics) ConsumeTraces(context.Context, ptrace.Traces) error {
	return nil
}

func (sm *signalToMetrics) ConsumeMetrics(context.Context, pmetric.Metrics) error {
	return nil
}

func (sm *signalToMetrics) ConsumeLogs(context.Context, plog.Logs) error {
	return nil
}
