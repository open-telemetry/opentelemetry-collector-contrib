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
	doneChan         chan struct{}
}

func newCollector(eventCh <-chan event, next consumer.Logs, logger *zap.Logger, obsrecv *receiverhelper.ObsReport, telemetryBuilder *metadata.TelemetryBuilder) *collector {
	return &collector{
		nextConsumer:     next,
		eventCh:          eventCh,
		logger:           logger,
		obsrecv:          obsrecv,
		telemetryBuilder: telemetryBuilder,
		doneChan:         make(chan struct{}),
	}
}

func (c *collector) Start() {
	go c.processEvents()
}

func (c *collector) Shutdown() {
	close(c.doneChan)
}

func (c *collector) processEvents() {
	for {
		select {
		case <-c.doneChan:
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
			c.telemetryBuilder.FluentRecordsGenerated.Add(context.Background(), int64(logRecordCount))
			obsCtx := c.obsrecv.StartLogsOp(context.Background())
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
