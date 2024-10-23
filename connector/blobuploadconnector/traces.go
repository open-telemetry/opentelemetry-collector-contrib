// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type passThroughTracesConnector struct {
	nextConsumer consumer.Traces
	component.StartFunc
	component.ShutdownFunc
}

func (c *passThroughTracesConnector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return c.nextConsumer.ConsumeTraces(ctx, td)
}

func (c *passThroughTracesConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func createTracesToTracesConnector(_ Deps) connector.CreateTracesToTracesFunc {
	return func(
		_ context.Context,
		_ connector.Settings,
		_ component.Config,
		nextConsumer consumer.Traces) (connector.Traces, error) {
		// TODO: implement actual, non-pass through implementation when
		// there is a config present with a non-empty "traces" config.
		return &passThroughTracesConnector{nextConsumer: nextConsumer}, nil
	}
}
