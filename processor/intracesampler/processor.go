// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package intracesampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intracesamplerprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

const (
	// The constants help translate user friendly percentages to numbers direct used in sampling.
	numHashBuckets        = 0x4000 // Using a power of 2 to avoid division.
	bitMaskHashBuckets    = numHashBuckets - 1
	percentageScaleFactor = numHashBuckets / 100.0
)

type inTraceSamplerProcessor struct {
	logger             *zap.Logger
	config             Config
	scopeLeavesMap     map[string]struct{}
	scaledSamplingRate uint32
	hashSeedBytes      []byte
}

func newInTraceSamplerSpansProcessor(ctx context.Context, set processor.CreateSettings, cfg *Config, nextConsumer consumer.Traces) (processor.Traces, error) {

	scopeLeavesMap := make(map[string]struct{})
	for i := 0; i < len(cfg.ScopeLeaves); i++ {
		scopeLeavesMap[cfg.ScopeLeaves[i]] = struct{}{}
	}

	its := &inTraceSamplerProcessor{
		logger:             set.Logger,
		config:             *cfg,
		scopeLeavesMap:     scopeLeavesMap,
		scaledSamplingRate: uint32(cfg.SamplingPercentage * percentageScaleFactor),
		hashSeedBytes:      i32tob(cfg.HashSeed),
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		its.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (its *inTraceSamplerProcessor) processTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	// Implementation will be added in followup PRs.
	// https://github.com/open-telemetry/opentelemetry-collector/blob/main/CONTRIBUTING.md#when-adding-a-new-component
	return td, nil
}
