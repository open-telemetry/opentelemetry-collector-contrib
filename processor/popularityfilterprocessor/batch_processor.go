// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package popularityfilterprocessor

import (
	"context"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"runtime"
	"sync"
	"time"
)

type popularityFilterProcessor struct {
	logger    *zap.Logger
	exportCtx context.Context
	timer     *time.Timer
	timeout   time.Duration
	// sendBatchMaxSize int
	tokens int

	newItem chan interface{}
	batch   batch

	shutdownC  chan struct{}
	goroutines sync.WaitGroup

	telemetryLevel configtelemetry.Level
}

type batch interface {
	// export the current batch
	export(ctx context.Context, returnBytes bool) (sentBatchSize int, sentBatchBytes int, err error)

	// itemCount returns the size of the current batch
	itemCount() int

	// add item to the current batch
	add(item interface{})
}

var _ consumer.Traces = (*popularityFilterProcessor)(nil)

var processorTagKey = tag.MustNewKey("processor")

func newBatchProcessor(set component.ProcessorCreateSettings, cfg *Config, batch batch, telemetryLevel configtelemetry.Level) (*popularityFilterProcessor, error) {
	exportCtx, err := tag.New(context.Background(), tag.Insert(processorTagKey, cfg.ID().String()))
	if err != nil {
		return nil, err
	}
	return &popularityFilterProcessor{
		logger:         set.Logger,
		exportCtx:      exportCtx,
		telemetryLevel: telemetryLevel,

		tokens:    int(cfg.Tokens),
		timeout:   cfg.Timeout,
		newItem:   make(chan interface{}, runtime.NumCPU()),
		batch:     batch,
		shutdownC: make(chan struct{}, 1),
	}, nil
}

func (bp *popularityFilterProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start is invoked during service startup.
func (bp *popularityFilterProcessor) Start(context.Context, component.Host) error {
	bp.goroutines.Add(1)
	go bp.startProcessingCycle()
	return nil
}

// Shutdown is invoked during service shutdown.
func (bp *popularityFilterProcessor) Shutdown(context.Context) error {
	close(bp.shutdownC)

	// Wait until all goroutines are done.
	bp.goroutines.Wait()
	return nil
}

func (bp *popularityFilterProcessor) startProcessingCycle() {
	defer bp.goroutines.Done()
	bp.timer = time.NewTimer(bp.timeout)
	for {
		select {
		case <-bp.shutdownC:
		DONE:
			for {
				select {
				case item := <-bp.newItem:
					bp.processItem(item)
				default:
					break DONE
				}
			}
			// This is the close of the channel
			if bp.batch.itemCount() > 0 {
				// TODO: Set a timeout on sendTraces or
				// make it cancellable using the context that Shutdown gets as a parameter
				bp.sendItems()
			}
			return
		case item := <-bp.newItem:
			if item == nil {
				continue
			}
			bp.processItem(item)
		case <-bp.timer.C:
			bp.sendItems()
			bp.resetTimer()
		}
	}
}

func (bp *popularityFilterProcessor) processItem(item interface{}) {
	bp.batch.add(item)
}

func (bp *popularityFilterProcessor) stopTimer() {
	if !bp.timer.Stop() {
		<-bp.timer.C
	}
}

func (bp *popularityFilterProcessor) resetTimer() {
	bp.timer.Reset(bp.timeout)
}

func (bp *popularityFilterProcessor) sendItems() {
	detailed := bp.telemetryLevel == configtelemetry.LevelDetailed
	_, _, err := bp.batch.export(bp.exportCtx, detailed)
	if err != nil {
		bp.logger.Warn("Sender failed", zap.Error(err))
	}
}

// ConsumeTraces implements TracesProcessor
func (bp *popularityFilterProcessor) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	bp.newItem <- td
	return nil
}

// newBatchTracesProcessor creates a new batch processor that batches traces by size or with timeout
func newTracesProcessor(set component.ProcessorCreateSettings, next consumer.Traces, cfg *Config, telemetryLevel configtelemetry.Level) (*popularityFilterProcessor, error) {
	return newBatchProcessor(set, cfg, newBatchTraces(next), telemetryLevel)
}

type batchTraces struct {
	nextConsumer consumer.Traces
	traceData    ptrace.Traces
	spanCount    int
	sizer        ptrace.Sizer
}

func newBatchTraces(nextConsumer consumer.Traces) *batchTraces {
	return &batchTraces{nextConsumer: nextConsumer, traceData: ptrace.NewTraces(), sizer: ptrace.NewProtoMarshaler().(ptrace.Sizer)}
}

// add updates current batchTraces by adding new TraceData object
func (bt *batchTraces) add(item interface{}) {
	td := item.(ptrace.Traces)
	newSpanCount := td.SpanCount()
	if newSpanCount == 0 {
		return
	}

	bt.spanCount += newSpanCount
	td.ResourceSpans().MoveAndAppendTo(bt.traceData.ResourceSpans())
}

func (bt *batchTraces) export(ctx context.Context, returnBytes bool) (int, int, error) {
	var req ptrace.Traces
	var sent int
	var bytes int

	req = bt.traceData
	sent = bt.spanCount
	bt.traceData = ptrace.NewTraces()
	bt.spanCount = 0

	if returnBytes {
		bytes = bt.sizer.TracesSize(req)
	}
	return sent, bytes, bt.nextConsumer.ConsumeTraces(ctx, req)
}

func (bt *batchTraces) itemCount() int {
	return bt.spanCount
}
