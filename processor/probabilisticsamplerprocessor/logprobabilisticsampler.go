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
	"sort"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

type severitySamplingRate struct {
	level              pdata.SeverityNumber
	scaledSamplingRate uint32
}

type logsamplerprocessor struct {
	samplingRates []*severitySamplingRate
	hashSeed      uint32
}

// newLogsProcessor returns a processor.LogsProcessor that will perform head sampling according to the given
// configuration.
func newLogsProcessor(nextConsumer consumer.Logs, cfg *Config) (component.LogsProcessor, error) {

	severitySamplingRates := []*severitySamplingRate{
		{level: pdata.SeverityNumberUNDEFINED, scaledSamplingRate: uint32(cfg.SamplingPercentage * percentageScaleFactor)},
	}
	sort.SliceStable(cfg.Severity, func(i, j int) bool {
		return severityTextToNum[cfg.Severity[i].Level] < severityTextToNum[cfg.Severity[j].Level]
	})
	for _, pair := range cfg.Severity {
		newRate := &severitySamplingRate{level: severityTextToNum[pair.Level],
			scaledSamplingRate: uint32(pair.SamplingPercentage * percentageScaleFactor),
		}
		severitySamplingRates = append(severitySamplingRates, newRate)
	}

	lsp := &logsamplerprocessor{
		samplingRates: severitySamplingRates,
		hashSeed:      cfg.HashSeed,
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

				// find the correct severity sampling level.
				var selectedSamplingRate *severitySamplingRate
				for _, ssr := range lsp.samplingRates {
					if ssr.level > l.SeverityNumber() {
						break
					}
					selectedSamplingRate = ssr
				}

				// Create an id for the log record by combining the timestamp and severity text.
				lidBytes := []byte(l.Timestamp().String() + l.SeverityText())
				sampled := hash(lidBytes[:], lsp.hashSeed)&bitMaskHashBuckets < selectedSamplingRate.scaledSamplingRate
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
