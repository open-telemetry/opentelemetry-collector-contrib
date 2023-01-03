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

package spanbatchsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanbatchsamplerprocessor"

import (
	"context"
	"runtime"
	"sync"

	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

type spanBatchSamplerProcessor struct {
	logger         *zap.Logger
	exportCtx      context.Context
	newItem        chan interface{}
	batch          batch
	tokensPerBatch int
	shutdownC      chan struct{}
	goroutines     sync.WaitGroup
	telemetryLevel configtelemetry.Level
	// spanFilter     filter // TODO add filter interface
}

type batch interface {
	// export the current batch
	export(context.Context, bool) (int, int, error)

	// itemCount returns the size of the current batch
	itemCount() int

	// add resource spans to the current batch
	addResourceSpans(ptrace.ResourceSpansSlice)
}

type batchTraces struct {
	nextConsumer consumer.Traces
	traceData    ptrace.Traces
	spanCount    int
	sizer        ptrace.Sizer
}

func (bt *batchTraces) addResourceSpans(rss ptrace.ResourceSpansSlice) {
	newSpanCount := rss.Len()
	if newSpanCount == 0 {
		return
	}

	bt.spanCount += newSpanCount
	rss.MoveAndAppendTo(bt.traceData.ResourceSpans())
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

func newBatchSamplerProcessor(set processor.CreateSettings, next consumer.Traces, cfg *Config, telemetryLevel configtelemetry.Level) (*spanBatchSamplerProcessor, error) {
	exportCtx, err := tag.New(context.Background())
	if err != nil {
		return nil, err
	}
	batch := newBatchTraces(next)

	// filter := newPopularitySpanFilter(cfg)

	return &spanBatchSamplerProcessor{
		logger:         set.Logger,
		exportCtx:      exportCtx,
		telemetryLevel: telemetryLevel,
		tokensPerBatch: cfg.TokensPerBatch,
		newItem:        make(chan interface{}, runtime.NumCPU()),
		batch:          batch,
		shutdownC:      make(chan struct{}, 1),
		// spanFilter:     filter, // TODO add filter interface
	}, nil
}

func newBatchTraces(nextConsumer consumer.Traces) *batchTraces {
	return &batchTraces{nextConsumer: nextConsumer, traceData: ptrace.NewTraces(), sizer: &ptrace.ProtoMarshaler{}}
}

func (sbf *spanBatchSamplerProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start is invoked during service startup.
func (sbf *spanBatchSamplerProcessor) Start(context.Context, component.Host) error {
	sbf.goroutines.Add(1)
	go sbf.startProcessingCycle()
	return nil
}

// Shutdown is invoked during service shutdown.
func (sbf *spanBatchSamplerProcessor) Shutdown(context.Context) error {
	close(sbf.shutdownC)

	// Wait until all goroutines are done.
	sbf.goroutines.Wait()
	return nil
}

func (sbf *spanBatchSamplerProcessor) addItem(item interface{}) {
	traces, cast := item.(ptrace.Traces)
	if !cast {
		sbf.logger.Warn("Failed getting traces from batch")
		return
	}
	// filteredResourceSpans := sbf.filterBatchTraces(traces) // TODO filtering occurs here
	sbf.batch.addResourceSpans(traces.ResourceSpans())
}

func (sbf *spanBatchSamplerProcessor) startProcessingCycle() {
	defer sbf.goroutines.Done()
	for {
		select {
		case <-sbf.shutdownC:
		DONE:
			for {
				select {
				case item := <-sbf.newItem:
					sbf.addItem(item)
				default:
					break DONE
				}
			}
			sbf.sendItems()
			return
		case item := <-sbf.newItem:
			if item == nil {
				sbf.logger.Warn("Received nil item")
				continue
			}
			sbf.addItem(item)
			sbf.sendItems()
		}
	}
}

func (sbf *spanBatchSamplerProcessor) sendItems() {
	if sbf.batch.itemCount() < 1 {
		return
	}
	detailed := sbf.telemetryLevel == configtelemetry.LevelDetailed
	_, _, err := sbf.batch.export(sbf.exportCtx, detailed)
	if err != nil {
		sbf.logger.Warn("Sender failed", zap.Error(err))
	}
}

func (sbf *spanBatchSamplerProcessor) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	sbf.newItem <- td
	return nil
}
