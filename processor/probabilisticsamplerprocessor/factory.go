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
	"sync"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	// The value of "type" trace-samplers in configuration.
	typeStr = "probabilistic_sampler"
	// The stability level of the processor.
	stability = component.StabilityLevelBeta
)

var onceMetrics sync.Once

// NewFactory returns a new factory for the Probabilistic sampler processor.
func NewFactory() processor.Factory {
	onceMetrics.Do(func() {
		// TODO: Handle this err
		_ = view.Register(SamplingProcessorMetricViews(configtelemetry.LevelNormal)...)
	})

	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
		processor.WithLogs(createLogsProcessor, component.StabilityLevelAlpha))
}

func createDefaultConfig() component.Config {
	return &Config{
		AttributeSource: defaultAttributeSource,
	}
}

// createTracesProcessor creates a trace processor based on this config.
func createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	return newTracesProcessor(ctx, set, cfg.(*Config), nextConsumer)
}

// createLogsProcessor creates a log processor based on this config.
func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	return newLogsProcessor(ctx, set, nextConsumer, cfg.(*Config))
}
