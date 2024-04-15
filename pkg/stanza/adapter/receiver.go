// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

type receiver struct {
	id     component.ID
	wg     sync.WaitGroup
	cancel context.CancelFunc

	pipe      pipeline.Pipeline
	emitter   *LogEmitter
	consumer  consumer.Logs
	converter *Converter
	logger    *zap.Logger
	obsrecv   *receiverhelper.ObsReport

	storageID     *component.ID
	storageClient storage.Client
}

// Ensure this receiver adheres to required interface
var _ rcvr.Logs = (*receiver)(nil)

// Start tells the receiver to start
func (r *receiver) Start(ctx context.Context, host component.Host) error {
	rctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.logger.Info("Starting stanza receiver")

	if err := r.setStorageClient(ctx, host); err != nil {
		return fmt.Errorf("storage client: %w", err)
	}

	if err := r.pipe.Start(r.storageClient); err != nil {
		return fmt.Errorf("start stanza: %w", err)
	}

	r.converter.Start()

	// Below we're starting 2 loops:
	// * one which reads all the logs produced by the emitter and then forwards
	//   them to converter
	// ...
	r.wg.Add(1)
	go r.emitterLoop(rctx)

	// ...
	// * second one which reads all the logs produced by the converter
	//   (aggregated by Resource) and then calls consumer to consumer them.
	r.wg.Add(1)
	go r.consumerLoop(rctx)

	// Those 2 loops are started in separate goroutines because batching in
	// the emitter loop can cause a flush, caused by either reaching the max
	// flush size or by the configurable ticker which would in turn cause
	// a set of log entries to be available for reading in converter's out
	// channel. In order to prevent backpressure, reading from the converter
	// channel and batching are done in those 2 goroutines.

	return nil
}

// emitterLoop reads the log entries produced by the emitter and batches them
// in converter.
func (r *receiver) emitterLoop(ctx context.Context) {
	defer r.wg.Done()

	// Don't create done channel on every iteration.
	doneChan := ctx.Done()
	for {
		select {
		case <-doneChan:
			r.logger.Debug("Receive loop stopped")
			return

		case e, ok := <-r.emitter.logChan:
			if !ok {
				continue
			}

			if err := r.converter.Batch(e); err != nil {
				r.logger.Error("Could not add entry to batch", zap.Error(err))
			}
		}
	}
}

// consumerLoop reads converter log entries and calls the consumer to consumer them.
func (r *receiver) consumerLoop(ctx context.Context) {
	defer r.wg.Done()

	// Don't create done channel on every iteration.
	doneChan := ctx.Done()
	pLogsChan := r.converter.OutChannel()
	for {
		select {
		case <-doneChan:
			r.logger.Debug("Consumer loop stopped")
			return

		case pLogs, ok := <-pLogsChan:
			if !ok {
				r.logger.Debug("Converter channel got closed")
				continue
			}
			obsrecvCtx := r.obsrecv.StartLogsOp(ctx)
			logRecordCount := pLogs.LogRecordCount()
			cErr := r.consumer.ConsumeLogs(ctx, pLogs)
			if cErr != nil {
				r.logger.Error("ConsumeLogs() failed", zap.Error(cErr))
			}
			r.obsrecv.EndLogsOp(obsrecvCtx, "stanza", logRecordCount, cErr)
		}
	}
}

// Shutdown is invoked during service shutdown
func (r *receiver) Shutdown(ctx context.Context) error {
	if r.cancel == nil {
		return nil
	}

	r.logger.Info("Stopping stanza receiver")
	pipelineErr := r.pipe.Stop()
	r.converter.Stop()
	r.cancel()
	r.wg.Wait()

	if r.storageClient != nil {
		clientErr := r.storageClient.Close(ctx)
		return multierr.Combine(pipelineErr, clientErr)
	}
	return pipelineErr
}
