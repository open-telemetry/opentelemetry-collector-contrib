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

// collector acts as an aggregator of LogRecords so that we don't have to
// generate as many plog.Logs instances...we can pre-batch the LogRecord
// instances from several Forward events into one to hopefully reduce
// allocations and GC overhead.
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
			rls := out.ResourceLogs().AppendEmpty()
			logSlice := rls.ScopeLogs().AppendEmpty().LogRecords()
			e.LogRecords().MoveAndAppendTo(logSlice)

			// Pull out anything waiting on the eventCh to get better
			// efficiency on LogResource allocations.
			c.fillBufferUntilChanEmpty(logSlice)

			logRecordCount := out.LogRecordCount()
			c.telemetryBuilder.FluentRecordsGenerated.Add(ctx, int64(logRecordCount))
			obsCtx := c.obsrecv.StartLogsOp(ctx)
			err := c.nextConsumer.ConsumeLogs(obsCtx, out)
			c.obsrecv.EndLogsOp(obsCtx, "fluent", logRecordCount, err)
		}
	}
}

func (c *collector) fillBufferUntilChanEmpty(dest plog.LogRecordSlice) {
	for {
		select {
		case e := <-c.eventCh:
			e.LogRecords().MoveAndAppendTo(dest)
		default:
			return
		}
	}
}
