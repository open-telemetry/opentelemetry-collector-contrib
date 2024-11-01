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

type tracesToTracesImpl struct {
	nextConsumer consumer.Traces
}

func (c *tracesToTracesImpl) Start(_ context.Context, _ component.Host) error {
	// TODO: implement
	return nil
}

func (c *tracesToTracesImpl) Shutdown(_ context.Context) error {
	// TODO: implement
	return nil
}

func (c *tracesToTracesImpl) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// TODO: implement
	return c.nextConsumer.ConsumeTraces(ctx, td)
}

func (c *tracesToTracesImpl) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func createTracesToTracesConnector(_ Deps) connector.CreateTracesToTracesFunc {
	return func(
		_ context.Context,
		_ connector.Settings,
		_ component.Config,
		nextConsumer consumer.Traces) (connector.Traces, error) {
		// TODO: implement
		return &tracesToTracesImpl{nextConsumer: nextConsumer}, nil
	}
}
