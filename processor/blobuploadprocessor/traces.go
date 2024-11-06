// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/blobuploadprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
)

type tracesProcessorImpl struct {
	nextConsumer consumer.Traces
}

func (c *tracesProcessorImpl) Start(_ context.Context, _ component.Host) error {
	// TODO: implement
	return nil
}

func (c *tracesProcessorImpl) Shutdown(_ context.Context) error {
	// TODO: implement
	return nil
}

func (c *tracesProcessorImpl) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// TODO: implement
	return c.nextConsumer.ConsumeTraces(ctx, td)
}

func (c *tracesProcessorImpl) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func createTracesProcessor(_ deps) processor.CreateTracesFunc {
	return func(
		_ context.Context,
		_ processor.Settings,
		_ component.Config,
		nextConsumer consumer.Traces) (processor.Traces, error) {
		// TODO: implement
		return &tracesProcessorImpl{nextConsumer: nextConsumer}, nil
	}
}
