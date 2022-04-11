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

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

type logsamplerprocessor struct {
	scaledSamplingRate uint32
	hashSeed           uint32
	traceIDEnabled     bool
	samplingSource     string
	samplingPriority   string
}

// newLogsProcessor returns a processor.LogsProcessor that will perform head sampling according to the given
// configuration.
func newLogsProcessor(nextConsumer consumer.Logs, cfg *Config) (component.LogsProcessor, error) {

	lsp := &logsamplerprocessor{
		scaledSamplingRate: uint32(cfg.SamplingPercentage * percentageScaleFactor),
		hashSeed:           cfg.HashSeed,
		traceIDEnabled:     cfg.TraceIDEnabled == nil || *cfg.TraceIDEnabled,
		samplingPriority:   cfg.SamplingPriority,
		samplingSource:     cfg.SamplingSource,
	}

	return processorhelper.NewLogsProcessor(
		cfg,
		nextConsumer,
		lsp.processLogs,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (lsp *logsamplerprocessor) processLogs(_ context.Context, ld pdata.Logs) (pdata.Logs, error) {
	ld.ResourceLogs().RemoveIf(func(rl pdata.ResourceLogs) bool {
		rl.ScopeLogs().RemoveIf(func(ill pdata.ScopeLogs) bool {
			ill.LogRecords().RemoveIf(func(l pdata.LogRecord) bool {

				// pick the sampling source.
				var lidBytes []byte
				if lsp.traceIDEnabled && !l.TraceID().IsEmpty() {
					value := l.TraceID().Bytes()
					lidBytes = value[:]
				}
				if lidBytes == nil && lsp.samplingSource != "" {
					if value, ok := l.Attributes().Get(lsp.samplingSource); ok {
						lidBytes = value.BytesVal()
					}
				}
				priority := lsp.scaledSamplingRate
				if lsp.samplingPriority != "" {
					if localPriority, ok := l.Attributes().Get(lsp.samplingPriority); ok {
						priority = uint32(localPriority.DoubleVal() * percentageScaleFactor)
					}
				}

				sampled := hash(lidBytes, lsp.hashSeed)&bitMaskHashBuckets < priority
				return !sampled
			})
			// Filter out empty ScopeLogs
			return ill.LogRecords().Len() == 0
		})
		// Filter out empty ResourceLogs
		return rl.ScopeLogs().Len() == 0
	})
	if ld.ResourceLogs().Len() == 0 {
		return ld, processorhelper.ErrSkipProcessingData
	}
	return ld, nil
}
