// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package eventhub // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/eventhub"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/trigger"
)

// LogsConsumer is a trigger.Consumer that unmarshals each Content message as logs
type LogsConsumer struct {
	unmarshaler plog.Unmarshaler
	nextLogs    consumer.Logs
}

// NewLogsConsumer returns a trigger.Consumer for Event Hub log bindings.
func NewLogsConsumer(unmarshaler plog.Unmarshaler, nextLogs consumer.Logs) *LogsConsumer {
	return &LogsConsumer{
		unmarshaler: unmarshaler,
		nextLogs:    nextLogs,
	}
}

// ConsumeEvents implements trigger.Consumer.
func (c *LogsConsumer) ConsumeEvents(ctx context.Context, req trigger.ParsedRequest) error {
	merged := plog.NewLogs()
	for i, msg := range req.Content {
		logs, err := c.unmarshaler.UnmarshalLogs(msg)
		if err != nil {
			return fmt.Errorf("unmarshal message %d: %w", i, err)
		}
		if logs.LogRecordCount() == 0 {
			continue
		}
		if len(req.Metadata) > 0 {
			trigger.AddMetadataToLogs(&logs, req.Metadata)
		}
		for j := 0; j < logs.ResourceLogs().Len(); j++ {
			logs.ResourceLogs().At(j).CopyTo(merged.ResourceLogs().AppendEmpty())
		}
	}
	if merged.LogRecordCount() == 0 {
		// Decision: Log events that result in zero records are treated
		// as anomalies and rejected as permanent errors.
		return errors.New("no logs to consume")
	}
	return c.nextLogs.ConsumeLogs(ctx, merged)
}
