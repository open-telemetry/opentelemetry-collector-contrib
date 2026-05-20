// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver/internal/metadata"
)

// collector aggregates LogRecords from multiple Forward events into a single
// plog.Logs per ConsumeLogs call for batching efficiency. Each event is placed
// in its own ResourceLogs to ensure resource attribute isolation.
type collector struct {
	nextConsumer     consumer.Logs
	eventCh          <-chan event
	logger           *zap.Logger
	obsrecv          *receiverhelper.ObsReport
	telemetryBuilder *metadata.TelemetryBuilder
}

func newCollector(eventCh <-chan event, next consumer.Logs, logger *zap.Logger, obsrecv *receiverhelper.ObsReport, telemetryBuilder *metadata.TelemetryBuilder) *collector {
	return &collector{
		nextConsumer:     next,
		eventCh:          eventCh,
		logger:           logger,
		obsrecv:          obsrecv,
		telemetryBuilder: telemetryBuilder,
	}
}

func (c *collector) Start(ctx context.Context) {
	go c.processEvents(ctx)
}

func (c *collector) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-c.eventCh:
			out := plog.NewLogs()
			appendEventToLogs(out, e)

			// Drain anything waiting on the eventCh into the same plog.Logs
			// for batching efficiency. Each event gets its own ResourceLogs to
			// ensure resource attribute isolation across events.
			c.fillBufferUntilChanEmpty(out)

			logRecordCount := out.LogRecordCount()
			c.telemetryBuilder.FluentRecordsGenerated.Add(ctx, int64(logRecordCount))
			obsCtx := c.obsrecv.StartLogsOp(ctx)
			err := c.nextConsumer.ConsumeLogs(obsCtx, out)
			c.obsrecv.EndLogsOp(obsCtx, "fluent", logRecordCount, err)
		}
	}
}

// appendEventToLogs places an event's log records into a new ResourceLogs
// within out, ensuring resource attribute isolation between events.
func appendEventToLogs(out plog.Logs, e event) {
	rls := out.ResourceLogs().AppendEmpty()
	logSlice := rls.ScopeLogs().AppendEmpty().LogRecords()
	e.LogRecords().MoveAndAppendTo(logSlice)
}

func (c *collector) fillBufferUntilChanEmpty(out plog.Logs) {
	for {
		select {
		case e := <-c.eventCh:
			appendEventToLogs(out, e)
		default:
			return
		}
	}
}
