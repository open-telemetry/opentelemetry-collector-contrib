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

type passThroughLogsConnector struct {
	nextConsumer consumer.Logs
}

func (c *passThroughLogsConnector) Start(_ context.Context, _ component.Host) error { return nil }
func (c *passThroughLogsConnector) Shutdown(_ context.Context) error                { return nil }

func (c *passThroughLogsConnector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return c.nextConsumer.ConsumeLogs(ctx, ld)
}

func (c *passThroughLogsConnector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func createLogsToLogsConnector(_ Deps) connector.CreateLogsToLogsFunc {
	return func(
		_ context.Context,
		_ connector.Settings,
		_ component.Config,
		nextConsumer consumer.Logs) (connector.Logs, error) {
		// TODO: implement actual, non-pass through implementation when
		// there is a config present with a non-empty "logs" config.
		return &passThroughLogsConnector{nextConsumer: nextConsumer}, nil
	}
}
