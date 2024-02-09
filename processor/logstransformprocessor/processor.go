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
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

type logsTransformProcessor struct {
	logger *zap.Logger
	config *Config

	consumer consumer.Logs

	pipe          *pipeline.DirectedPipeline
	firstOperator operator.Operator
	emitter       *adapter.LogEmitter
	converter     *adapter.Converter
	fromConverter *adapter.FromPdataConverter
	shutdownFns   []component.ShutdownFunc
}

func newProcessor(config *Config, nextConsumer consumer.Logs, logger *zap.Logger) (*logsTransformProcessor, error) {
	p := &logsTransformProcessor{
		logger:   logger,
		config:   config,
		consumer: nextConsumer,
	}

	baseCfg := p.config.BaseConfig

	p.emitter = adapter.NewLogEmitter(p.logger.Sugar())
	pipe, err := pipeline.Config{
		Operators:     baseCfg.Operators,
		DefaultOutput: p.emitter,
	}.Build(p.logger.Sugar())
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
	ltp.logger.Info("Stopping logs transform processor")
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
	// create all objects before starting them, since the loops (consumerLoop, converterLoop) depend on these converters not being nil.
	ltp.converter = adapter.NewConverter(ltp.logger)

	wkrCount := int(math.Max(1, float64(runtime.NumCPU())))
	ltp.fromConverter = adapter.NewFromPdataConverter(wkrCount, ltp.logger)

	// data flows in this order:
	// ConsumeLogs: receives logs and forwards them for conversion to stanza format ->
	// fromConverter: converts logs to stanza format ->
	// converterLoop: forwards converted logs to the stanza pipeline ->
	// pipeline: performs user configured operations on the logs ->
	// emitterLoop: forwards output stanza logs for conversion to OTLP ->
	// converter: converts stanza logs to OTLP ->
	// consumerLoop: sends the converted OTLP logs to the next consumer
	//
	// We should start these components in reverse order of the data flow, then stop them in order of the data flow,
	// in order to allow for pipeline draining.
	ltp.startConsumerLoop(ctx)
	ltp.startConverter()
	ltp.startEmitterLoop(ctx)
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

	ltp.shutdownFns = append(ltp.shutdownFns, func(ctx context.Context) error {
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

	ltp.shutdownFns = append(ltp.shutdownFns, func(ctx context.Context) error {
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

	ltp.shutdownFns = append(ltp.shutdownFns, func(ctx context.Context) error {
		return ltp.pipe.Stop()
	})

	pipelineOperators := ltp.pipe.Operators()
	if len(pipelineOperators) == 0 {
		return errors.New("processor requires at least one operator to be configured")
	}
	ltp.firstOperator = pipelineOperators[0]

	return nil
}

// startEmitterLoop starts the loop which reads all the logs modified by the pipeline and then forwards
// them to converter
func (ltp *logsTransformProcessor) startEmitterLoop(ctx context.Context) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go ltp.emitterLoop(ctx, wg)

	ltp.shutdownFns = append(ltp.shutdownFns, func(ctx context.Context) error {
		wg.Wait()
		return nil
	})
}

func (ltp *logsTransformProcessor) startConverter() {
	ltp.converter.Start()

	ltp.shutdownFns = append(ltp.shutdownFns, func(ctx context.Context) error {
		ltp.converter.Stop()
		return nil
	})
}

// startConsumerLoop starts the loop which reads all the logs produced by the converter
// (aggregated by Resource) and then places them on the next consumer
func (ltp *logsTransformProcessor) startConsumerLoop(ctx context.Context) {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go ltp.consumerLoop(ctx, wg)

	ltp.shutdownFns = append(ltp.shutdownFns, func(ctx context.Context) error {
		wg.Wait()
		return nil
	})
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
			ltp.logger.Debug("converter loop stopped")
			return

		case entries, ok := <-ltp.fromConverter.OutChannel():
			if !ok {
				ltp.logger.Debug("fromConverter channel got closed")
				return
			}

			for _, e := range entries {
				// Add item to the first operator of the pipeline manually
				if err := ltp.firstOperator.Process(ctx, e); err != nil {
					ltp.logger.Error("processor encountered an issue with the pipeline", zap.Error(err))
					break
				}
			}
		}
	}
}

// emitterLoop reads the log entries produced by the emitter and batches them
// in converter.
func (ltp *logsTransformProcessor) emitterLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			ltp.logger.Debug("emitter loop stopped")
			return
		case e, ok := <-ltp.emitter.OutChannel():
			if !ok {
				ltp.logger.Debug("emitter channel got closed")
				return
			}

			if err := ltp.converter.Batch(e); err != nil {
				ltp.logger.Error("processor encountered an issue with the converter", zap.Error(err))
			}
		}
	}
}

// consumerLoop reads converter log entries and calls the consumer to consumer them.
func (ltp *logsTransformProcessor) consumerLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			ltp.logger.Debug("consumer loop stopped")
			return

		case pLogs, ok := <-ltp.converter.OutChannel():
			if !ok {
				ltp.logger.Debug("converter channel got closed")
				return
			}

			if err := ltp.consumer.ConsumeLogs(ctx, pLogs); err != nil {
				ltp.logger.Error("processor encountered an issue with next consumer", zap.Error(err))
			}
		}
	}
}
