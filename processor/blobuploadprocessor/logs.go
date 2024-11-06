// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/blobuploadprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
)

type logsProcessorImpl struct {
	nextConsumer consumer.Logs
}

func (c *logsProcessorImpl) Start(_ context.Context, _ component.Host) error {
	// TODO: implement
	return nil
}

func (c *logsProcessorImpl) Shutdown(_ context.Context) error {
	// TODO: implement
	return nil
}

func (c *logsProcessorImpl) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// TODO: implement
	return c.nextConsumer.ConsumeLogs(ctx, ld)
}

func (c *logsProcessorImpl) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func createLogsProcessor(_ deps) processor.CreateLogsFunc {
	return func(
		_ context.Context,
		_ processor.Settings,
		_ component.Config,
		nextConsumer consumer.Logs) (processor.Logs, error) {
		// TODO: implement
		return &logsProcessorImpl{nextConsumer: nextConsumer}, nil
	}
}
