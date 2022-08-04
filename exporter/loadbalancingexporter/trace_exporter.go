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

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

var _ component.TracesExporter = (*traceExporterImp)(nil)

var (
	errNoTracesInBatch = errors.New("no traces were found in the batch")
)

type traceExporterImp struct {
	loadBalancer loadBalancer

	stopped    bool
	shutdownWg sync.WaitGroup

	logger *zap.Logger

	// add batch config
	enableBatch   bool
	batchTimeout  time.Duration
	sendBatchSize int

	newTraceChan chan ptrace.Traces
	shutdownChan chan struct{}

	timer             *time.Timer
	endpointBatchDict map[string]*batchTraces
}

// Create new traces exporter
func newTracesExporter(params component.ExporterCreateSettings, cfg config.Exporter) (*traceExporterImp, error) {
	exporterFactory := otlpexporter.NewFactory()

	lb, err := newLoadBalancer(params, cfg, func(ctx context.Context, endpoint string) (component.Exporter, error) {
		oCfg := buildExporterConfig(cfg.(*Config), endpoint)
		return exporterFactory.CreateTracesExporter(ctx, params, &oCfg)
	})
	if err != nil {
		return nil, err
	}

	traceConfig := cfg.(*Config)

	return &traceExporterImp{
		loadBalancer:      lb,
		logger:            params.Logger,
		enableBatch:       traceConfig.Batch.Enable,
		batchTimeout:      traceConfig.Batch.Timeout,
		sendBatchSize:     traceConfig.Batch.SendBatchSize,
		newTraceChan:      make(chan ptrace.Traces, runtime.NumCPU()),
		shutdownChan:      make(chan struct{}, 1),
		endpointBatchDict: map[string]*batchTraces{},
	}, nil
}

func buildExporterConfig(cfg *Config, endpoint string) otlpexporter.Config {
	oCfg := cfg.Protocol.OTLP
	oCfg.ExporterSettings = config.NewExporterSettings(config.NewComponentID("otlp"))
	oCfg.Endpoint = endpoint
	return oCfg
}

func (e *traceExporterImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *traceExporterImp) Start(ctx context.Context, host component.Host) error {
	if e.enableBatch {
		e.shutdownWg.Add(1)
		go e.startProcessingCycle()
	}

	return e.loadBalancer.Start(ctx, host)
}

func (e *traceExporterImp) Shutdown(context.Context) error {
	if e.enableBatch {
		close(e.shutdownChan)
	}

	e.stopped = true
	e.shutdownWg.Wait()
	return nil
}

func (e *traceExporterImp) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var errs error
	batches := batchpersignal.SplitTraces(td)
	for _, batch := range batches {
		errs = multierr.Append(errs, e.consumeTrace(ctx, batch))
	}

	return errs
}

func (e *traceExporterImp) consumeTrace(ctx context.Context, td ptrace.Traces) error {
	if e.enableBatch {
		e.newTraceChan <- td
		return nil
	}

	traceID := traceIDFromTraces(td)
	if traceID == pcommon.InvalidTraceID() {
		return errNoTracesInBatch
	}

	endpoint := e.loadBalancer.Endpoint(traceID)
	exp, err := e.loadBalancer.Exporter(endpoint)
	if err != nil {
		return err
	}

	te, ok := exp.(component.TracesExporter)
	if !ok {
		expectType := (*component.TracesExporter)(nil)
		return fmt.Errorf("expected %T but got %T", expectType, exp)
	}

	start := time.Now()
	err = te.ConsumeTraces(ctx, td)
	duration := time.Since(start)
	ctx, _ = tag.New(ctx, tag.Upsert(tag.MustNewKey("endpoint"), endpoint))

	if err == nil {
		sCtx, _ := tag.New(ctx, tag.Upsert(tag.MustNewKey("success"), "true"))
		stats.Record(sCtx, mBackendLatency.M(duration.Milliseconds()))
	} else {
		fCtx, _ := tag.New(ctx, tag.Upsert(tag.MustNewKey("success"), "false"))
		stats.Record(fCtx, mBackendLatency.M(duration.Milliseconds()))
	}

	return err
}

func traceIDFromTraces(td ptrace.Traces) pcommon.TraceID {
	rs := td.ResourceSpans()
	if rs.Len() == 0 {
		return pcommon.InvalidTraceID()
	}

	ils := rs.At(0).ScopeSpans()
	if ils.Len() == 0 {
		return pcommon.InvalidTraceID()
	}

	spans := ils.At(0).Spans()
	if spans.Len() == 0 {
		return pcommon.InvalidTraceID()
	}

	return spans.At(0).TraceID()
}

func (e *traceExporterImp) startProcessingCycle() {
	defer e.shutdownWg.Done()
	e.timer = time.NewTimer(e.batchTimeout)
	for {
		select {
		case <-e.shutdownChan:
		DONE:
			for {
				select {
				case oneTrace := <-e.newTraceChan:
					err := e.processTrace(oneTrace)
					if err != nil {

					}
				default:
					break DONE
				}
			}

			e.sendAll()
			return
		case oneTrace := <-e.newTraceChan:
			err := e.processTrace(oneTrace)
			if err != nil {
				e.logger.Error("process one trace occur error", zap.Error(err))
			}
		case <-e.timer.C:
			e.sendAll()
			e.resetTimer()
		}
	}
}

func (e *traceExporterImp) stopTimer() {
	if !e.timer.Stop() {
		<-e.timer.C
	}
}

func (e *traceExporterImp) resetTimer() {
	e.timer.Reset(e.batchTimeout)
}

type batchTraces struct {
	traceData ptrace.Traces
	spanCount int
}

func (e *traceExporterImp) processTrace(td ptrace.Traces) error {
	newSpanCount := td.SpanCount()
	if newSpanCount == 0 {
		return nil
	}

	traceID := traceIDFromTraces(td)
	if traceID == pcommon.InvalidTraceID() {
		return errNoTracesInBatch
	}

	endpoint := e.loadBalancer.Endpoint(traceID)

	batchTrace, ok := e.endpointBatchDict[endpoint]
	if !ok {
		e.endpointBatchDict[endpoint] = &batchTraces{
			traceData: ptrace.NewTraces(),
			spanCount: 0,
		}

		batchTrace = e.endpointBatchDict[endpoint]
	}

	batchTrace.spanCount += newSpanCount

	td.ResourceSpans().MoveAndAppendTo(batchTrace.traceData.ResourceSpans())

	// send trace
	sent := false
	var err error = nil
	if batchTrace.spanCount >= e.sendBatchSize {
		sent = true
		err = e.sendTraces(endpoint, batchTrace)
	}

	if sent {
		e.stopTimer()
		e.resetTimer()
	}

	return err
}

func (e *traceExporterImp) sendTraces(endpoint string, traces *batchTraces) error {
	// if backends changed, batched data will export to another backend
	// if you don't accept that, do not use batch mode(with low throughput, because trace data will expose exactly one by one)
	exp, err := e.loadBalancer.Exporter(endpoint)
	if err != nil {
		return err
	}

	te, ok := exp.(component.TracesExporter)
	if !ok {
		expectType := (*component.TracesExporter)(nil)
		return fmt.Errorf("expected %T but got %T", expectType, exp)
	}

	err = te.ConsumeTraces(context.Background(), traces.traceData)
	traces.traceData = ptrace.NewTraces()
	traces.spanCount = 0

	return err
}

// batch timeout or shutdown ,send all trace data
func (e *traceExporterImp) sendAll() {
	for endpoint, traces := range e.endpointBatchDict {
		if traces.spanCount > 0 {
			err := e.sendTraces(endpoint, traces)
			if err != nil {
				e.logger.Error("send all trace data occur error", zap.String("endpoint", endpoint), zap.Error(err))
			}
		}
	}
}
