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
	"math"
	"runtime"
	"sync"

	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/opentelemetry-collector-contrib/internal/stanza"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza"
	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/pipeline"
)

type logsTransformProcessor struct {
	wg sync.WaitGroup

	logger        *zap.Logger
	config        *Config
	pipeline      *pipeline.Pipeline
	emitter       *stanza.LogEmitter
	converter     *stanza.Converter
	fromConverter *stanza.FromPdataConverter

	outputChannel chan pdata.Logs
}

func (ltp *logsTransformProcessor) init(ctx context.Context) error {
	baseCfg := ltp.config.BaseConfig
	operators, err := baseCfg.decodeOperatorConfigs()
	if err != nil {
		return err
	}

	if len(operators) == 0 {
		return errors.New("No operators were configured for this logs transform processor")
	}

	emitterOpts := []stanza.LogEmitterOption{
		stanza.LogEmitterWithLogger(ltp.logger),
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
	}.Build(ltp.logger)
	if err != nil {
		return err
	}

	ltp.pipeline = pipe

	wkrCount := int(math.Max(1, float64(runtime.NumCPU()/4)))
	if baseCfg.Converter.WorkerCount > 0 {
		wkrCount = baseCfg.Convert.WorkerCount
	}
	opts := []stanza.ConverterOption{
		stanza.WithLogger(ltp.logger),
		stanza.WithWorkerCount(wkrCount),
	}

	ltp.converter = stanza.NewConverter(opts...)

	ltp.fromConverter = stanza.NewFromPdataConverter(wkrCount, ltp.logger)

	ltp.outputChannel = make(chan pdata.Logs)
	ltp.converter.Start()
	ltp.fromConverter.Start()

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

func (ltp *logsTransformProcessor) processLogs(ctx context.Context, ld pdata.Logs) (pdata.Logs, error) {
	// Add the logs to the chain
	ltp.fromConverter.Batch(ld)

	doneChan := ctx.Done()
	for {
		select {
		case <-doneChan:
			ltp.logger.Debug("loop stopped")
			return ld, errors.New("processor interrupted")
		case pLogs, ok := <-ltp.outputChannel:
			if !ok {
				return ld, errors.New("processor encountered an issue receiving logs from stanza operators pipeline")
			}

			return pLogs, nil
		}
	}
}

func (ltp *logsTransformProcessor) convertLogsToEntries(ctx context.Context, ld pdata.Logs) []*entry.Entry {
	return stanza.ConvertFrom(ld)
}

// converterLoop reads the log entries produced by the fromConverter and sends them
// into the pipeline
func (ltp *logsTransformProcessor) converterLoop(ctx context.Context) {
	defer ltp.wg.Done()

	// Don't create done channel on every iteration.
	doneChan := ctx.Done()
	for {
		select {
		case <-doneChan:
			ltp.logger.Debug("converter loop stopped")
			return

		case entries, ok := <-ltp.fromConverter.OutChannel():
			if !ok {
				ltp.logger.Debug("fromConverter channel got closed")
				continue
			}

			for _, e := range entries {
				ltp.pipeline.Operators()[0].Process(ctx, e)
			}
		}
	}
}

// emitterLoop reads the log entries produced by the emitter and batches them
// in converter.
func (ltp *logsTransformProcessor) emitterLoop(ctx context.Context) {
	defer ltp.wg.Done()

	// Don't create done channel on every iteration.
	doneChan := ctx.Done()
	for {
		select {
		case <-doneChan:
			ltp.logger.Debug("emitter loop stopped")
			return

		case e, ok := <-ltp.emitter.OutChannel():
			if !ok {
				ltp.logger.Debug("emitter channel got closed")
				continue
			}

			ltp.converter.Batch(e)
		}
	}
}

// consumerLoop reads converter log entries and calls the consumer to consumer them.
func (ltp *logsTransformProcessor) consumerLoop(ctx context.Context) {
	defer ltp.wg.Done()

	// Don't create done channel on every iteration.
	doneChan := ctx.Done()
	pLogsChan := ltp.converter.OutChannel()
	for {
		select {
		case <-doneChan:
			ltp.logger.Debug("consumer loop stopped")
			return

		case pLogs, ok := <-pLogsChan:
			if !ok {
				ltp.logger.Debug("converter channel got closed")
				continue
			}

			ltp.outputChannel <- pLogs
		}
	}
}
