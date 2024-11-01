// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadconnector

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
)

type logsToLogsImpl struct {
	nextConsumer consumer.Logs
}

func (c *logsToLogsImpl) Start(_ context.Context, _ component.Host) error {
	// TODO: implement
	return nil
}

func (c *logsToLogsImpl) Shutdown(_ context.Context) error {
	// TODO: implement
	return nil
}

func (c *logsToLogsImpl) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// TODO: implement
	return c.nextConsumer.ConsumeLogs(ctx, ld)
}

func (c *logsToLogsImpl) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func createLogsToLogsConnector(_ Deps) connector.CreateLogsToLogsFunc {
	return func(
		_ context.Context,
		_ connector.Settings,
		_ component.Config,
		nextConsumer consumer.Logs) (connector.Logs, error) {
		// TODO: implement
		return &logsToLogsImpl{nextConsumer: nextConsumer}, nil
	}
}
