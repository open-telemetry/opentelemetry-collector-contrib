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

package cascadingfilterprocessor

import (
	"context"
	"time"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"

	cfconfig "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/config"
)

const (
	// The value of "type" Cascading Filter in configuration.
	typeStr = "cascading_filter"
)

var (
	defaultProbabilisticFilteringRatio = float32(0.2)
)

// NewFactory returns a new factory for the Cascading Filter processor.
func NewFactory() component.ProcessorFactory {
	// TODO: this is hardcoding the metrics level and skips error handling
	_ = view.Register(CascadingFilterMetricViews(configtelemetry.LevelNormal)...)

	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithTraces(createTraceProcessor))
}

func createDefaultConfig() config.Processor {
	return &cfconfig.Config{
		ProcessorSettings: &config.ProcessorSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
		DecisionWait:                30 * time.Second,
		NumTraces:                   50000,
		SpansPerSecond:              1500,
		ProbabilisticFilteringRatio: &defaultProbabilisticFilteringRatio,
	}
}

func createTraceProcessor(
	_ context.Context,
	params component.ProcessorCreateParams,
	cfg config.Processor,
	nextConsumer consumer.Traces,
) (component.TracesProcessor, error) {
	tCfg := cfg.(*cfconfig.Config)
	return newTraceProcessor(params.Logger, nextConsumer, *tCfg)
}
