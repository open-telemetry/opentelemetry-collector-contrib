// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstransformprocessor"

import (
	"context"
	"errors"
	"math"
	"runtime"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

type logsTransformProcessor struct {
	set    component.TelemetrySettings
	config *Config

	consumer consumer.Logs

	pipe          *pipeline.DirectedPipeline
	firstOperator operator.Operator
	emitter       helper.LogEmitter
	fromConverter *adapter.FromPdataConverter
	shutdownFns   []component.ShutdownFunc
}

func newProcessor(config *Config, nextConsumer consumer.Logs, set component.TelemetrySettings) (*logsTransformProcessor, error) {
	p := &logsTransformProcessor{
		set:      set,
		config:   config,
		consumer: nextConsumer,
	}

	baseCfg := p.config.BaseConfig

	p.emitter = helper.NewBatchingLogEmitter(p.set, p.consumeStanzaLogEntries)
	pipe, err := pipeline.Config{
		Operators:     baseCfg.Operators,
		DefaultOutput: p.emitter,
	}.Build(set)
	if err != nil {
		return nil, err
	}

	p.pipe = pipe

	return p, nil
}

func (ltp *logsTransformProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (ltp *logsTransformProcessor) Shutdown(ctx context.Context) error {
	ltp.set.Logger.Info("Stopping logs transform processor")
	// We call the shutdown functions in reverse order, so that the last thing we started
	// is stopped first.
	for i := len(ltp.shutdownFns) - 1; i >= 0; i-- {
		fn := ltp.shutdownFns[i]

		if err := fn(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (ltp *logsTransformProcessor) Start(ctx context.Context, _ component.Host) error {
	wkrCount := int(math.Max(1, float64(runtime.NumCPU())))
	ltp.fromConverter = adapter.NewFromPdataConverter(ltp.set, wkrCount)

	// data flows in this order:
	// ConsumeLogs: receives logs and forwards them for conversion to stanza format ->
	// fromConverter: converts logs to stanza format ->
	// converterLoop: forwards converted logs to the stanza pipeline ->
	// pipeline: performs user configured operations on the logs ->
	// transformProcessor: receives []*entry.Entries, converts them to plog.Logs and sends the converted OTLP logs to the next consumer
	//
	// We should start these components in reverse order of the data flow, then stop them in order of the data flow,
	// in order to allow for pipeline draining.
	err := ltp.startPipeline()
	if err != nil {
		return err
	}
	ltp.startConverterLoop(ctx)
	ltp.startFromConverter()

	return nil
}

func (ltp *logsTransformProcessor) startFromConverter() {
	ltp.fromConverter.Start()

	ltp.shutdownFns = append(ltp.shutdownFns, func(_ context.Context) error {
		ltp.fromConverter.Stop()
		return nil
	})
}

// startConverterLoop starts the converter loop, which reads all the logs translated by the fromConverter and then forwards
// them to pipeline
func (ltp *logsTransformProcessor) startConverterLoop(ctx context.Context) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go ltp.converterLoop(ctx, wg)

	ltp.shutdownFns = append(ltp.shutdownFns, func(_ context.Context) error {
		wg.Wait()
		return nil
	})
}

func (ltp *logsTransformProcessor) startPipeline() error {
	// There is no need for this processor to use storage
	err := ltp.pipe.Start(storage.NewNopClient())
	if err != nil {
		return err
	}

	ltp.shutdownFns = append(ltp.shutdownFns, func(_ context.Context) error {
		return ltp.pipe.Stop()
	})

	pipelineOperators := ltp.pipe.Operators()
	if len(pipelineOperators) == 0 {
		return errors.New("processor requires at least one operator to be configured")
	}
	ltp.firstOperator = pipelineOperators[0]

	return nil
}

func (ltp *logsTransformProcessor) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	// Add the logs to the chain
	return ltp.fromConverter.Batch(ld)
}

// converterLoop reads the log entries produced by the fromConverter and sends them
// into the pipeline
func (ltp *logsTransformProcessor) converterLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			ltp.set.Logger.Debug("converter loop stopped")
			return

		case entries, ok := <-ltp.fromConverter.OutChannel():
			if !ok {
				ltp.set.Logger.Debug("fromConverter channel got closed")
				return
			}

			for _, e := range entries {
				// Add item to the first operator of the pipeline manually
				if err := ltp.firstOperator.Process(ctx, e); err != nil {
					ltp.set.Logger.Error("processor encountered an issue with the pipeline", zap.Error(err))
					break
				}
			}
		}
	}
}

func (ltp *logsTransformProcessor) consumeStanzaLogEntries(ctx context.Context, entries []*entry.Entry) {
	pLogs := adapter.ConvertEntries(entries)
	if err := ltp.consumer.ConsumeLogs(ctx, pLogs); err != nil {
		ltp.set.Logger.Error("processor encountered an issue with next consumer", zap.Error(err))
	}
}
