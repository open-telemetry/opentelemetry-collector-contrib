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

package logstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstransformprocessor"

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
	"gonum.org/v1/gonum/graph/topo"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza"
)

type outputType struct {
	logs plog.Logs
	err  error
}

type logsTransformProcessor struct {
	logger *zap.Logger
	config *Config
	id     config.ComponentID

	pipe          *pipeline.DirectedPipeline
	firstOperator operator.Operator
	emitter       *stanza.LogEmitter
	converter     *stanza.Converter
	fromConverter *stanza.FromPdataConverter
	wg            sync.WaitGroup
	outputChannel chan outputType
}

func (ltp *logsTransformProcessor) Shutdown(ctx context.Context) error {
	ltp.logger.Info("Stopping logs transform processor")
	pipelineErr := ltp.pipe.Stop()
	ltp.converter.Stop()
	ltp.fromConverter.Stop()
	ltp.wg.Wait()

	return pipelineErr
}

func (ltp *logsTransformProcessor) Start(ctx context.Context, host component.Host) error {
	baseCfg := ltp.config.BaseConfig
	operators, err := baseCfg.DecodeOperatorConfigs()
	if err != nil {
		return err
	}

	emitterOpts := []stanza.LogEmitterOption{
		stanza.LogEmitterWithLogger(ltp.logger.Sugar()),
	}
	if baseCfg.Converter.MaxFlushCount > 0 {
		emitterOpts = append(emitterOpts, stanza.LogEmitterWithMaxBatchSize(baseCfg.Converter.MaxFlushCount))
	}
	if baseCfg.Converter.FlushInterval > 0 {
		emitterOpts = append(emitterOpts, stanza.LogEmitterWithFlushInterval(baseCfg.Converter.FlushInterval))
	}
	ltp.emitter = stanza.NewLogEmitter(emitterOpts...)
	pipe, err := pipeline.Config{
		Operators:     operators,
		DefaultOutput: ltp.emitter,
	}.Build(ltp.logger.Sugar())
	if err != nil {
		return err
	}

	storageClient, err := stanza.GetStorageClient(ctx, ltp.id, component.KindProcessor, host)
	if err != nil {
		return err
	}

	err = pipe.Start(stanza.GetPersister(storageClient))
	if err != nil {
		return err
	}

	ltp.pipe = pipe

	orderedNodes, err := topo.Sort(pipe.Graph)
	if err != nil {
		return err
	}
	if len(orderedNodes) == 0 {
		return errors.New("processor requires at least one operator to be configured")
	}
	ltp.firstOperator = orderedNodes[0].(pipeline.OperatorNode).Operator()

	wkrCount := int(math.Max(1, float64(runtime.NumCPU())))
	if baseCfg.Converter.WorkerCount > 0 {
		wkrCount = baseCfg.Converter.WorkerCount
	}

	ltp.converter = stanza.NewConverter(
		stanza.WithLogger(ltp.logger),
		stanza.WithWorkerCount(wkrCount),
	)
	ltp.converter.Start()

	ltp.fromConverter = stanza.NewFromPdataConverter(wkrCount, ltp.logger)
	ltp.fromConverter.Start()

	ltp.outputChannel = make(chan outputType)

	// Below we're starting 3 loops:
	// * first which reads all the logs translated by the fromConverter and then forwards
	//   them to pipeline
	// ...
	ltp.wg.Add(1)
	go ltp.converterLoop(ctx)

	// * second which reads all the logs modified by the pipeline and then forwards
	//   them to converter
	// ...
	ltp.wg.Add(1)
	go ltp.emitterLoop(ctx)

	// ...
	// * third which reads all the logs produced by the converter
	//   (aggregated by Resource) and then places them on the outputChannel
	ltp.wg.Add(1)
	go ltp.consumerLoop(ctx)

	return nil
}

func (ltp *logsTransformProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	// Add the logs to the chain
	err := ltp.fromConverter.Batch(ld)
	if err != nil {
		return ld, err
	}

	doneChan := ctx.Done()
	for {
		select {
		case <-doneChan:
			ltp.logger.Debug("loop stopped")
			return ld, errors.New("processor interrupted")
		case output, ok := <-ltp.outputChannel:
			if !ok {
				return ld, errors.New("processor encountered an issue receiving logs from stanza operators pipeline")
			}
			if output.err != nil {
				return ld, err
			}

			return output.logs, nil
		}
	}
}

// converterLoop reads the log entries produced by the fromConverter and sends them
// into the pipeline
func (ltp *logsTransformProcessor) converterLoop(ctx context.Context) {
	defer ltp.wg.Done()

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
					ltp.outputChannel <- outputType{err: fmt.Errorf("processor encountered an issue with the pipeline: %w", err)}
					break
				}
			}
		}
	}
}

// emitterLoop reads the log entries produced by the emitter and batches them
// in converter.
func (ltp *logsTransformProcessor) emitterLoop(ctx context.Context) {
	defer ltp.wg.Done()

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
				ltp.outputChannel <- outputType{err: fmt.Errorf("processor encountered an issue with the converter: %w", err)}
			}
		}
	}
}

// consumerLoop reads converter log entries and calls the consumer to consumer them.
func (ltp *logsTransformProcessor) consumerLoop(ctx context.Context) {
	defer ltp.wg.Done()

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

			ltp.outputChannel <- outputType{logs: pLogs, err: nil}
		}
	}
}
