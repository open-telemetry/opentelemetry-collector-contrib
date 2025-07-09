// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type traceToTraceConnector struct {
	logger         *zap.Logger
	tracesConsumer consumer.Traces // the next component in the pipeline to ingest traces after connector
}

func newTraceToTraceConnector(logger *zap.Logger, nextConsumer consumer.Traces) *traceToTraceConnector {
	logger.Info("Building datadog connector for trace to trace")
	return &traceToTraceConnector{
		logger:         logger,
		tracesConsumer: nextConsumer,
	}
}

// Start implements the component interface.
func (c *traceToTraceConnector) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown implements the component interface.
func (c *traceToTraceConnector) Shutdown(_ context.Context) error {
	return nil
}

// Capabilities implements the consumer interface.
// tells use whether the component(connector) will mutate the data passed into it. if set to true the connector does modify the data
func (c *traceToTraceConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer interface.
func (c *traceToTraceConnector) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	return c.tracesConsumer.ConsumeTraces(ctx, traces)
}
