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

package groupbyattrsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor"

import (
	"context"
	"sync"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor/internal/metadata"
)

var (
	consumerCapabilities = consumer.Capabilities{MutatesData: true}
)

var once sync.Once

// NewFactory returns a new factory for the Filter processor.
func NewFactory() processor.Factory {
	once.Do(func() {
		// TODO: as with other -contrib factories registering metrics, this is causing the error being ignored
		_ = view.Register(MetricViews()...)
	})

	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability))
}

// createDefaultConfig creates the default configuration for the processor.
func createDefaultConfig() component.Config {
	return &Config{
		GroupByKeys: []string{},
	}
}

func createGroupByAttrsProcessor(logger *zap.Logger, attributes []string) *groupByAttrsProcessor {
	var nonEmptyAttributes []string
	presentAttributes := make(map[string]struct{})

	for _, str := range attributes {
		if str != "" {
			_, isPresent := presentAttributes[str]
			if isPresent {
				logger.Warn("A grouping key is already present", zap.String("key", str))
			} else {
				nonEmptyAttributes = append(nonEmptyAttributes, str)
				presentAttributes[str] = struct{}{}
			}
		}
	}

	return &groupByAttrsProcessor{logger: logger, groupByKeys: nonEmptyAttributes}
}

// createTracesProcessor creates a trace processor based on this config.
func createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces) (processor.Traces, error) {

	oCfg := cfg.(*Config)
	gap := createGroupByAttrsProcessor(set.Logger, oCfg.GroupByKeys)

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		gap.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities))
}

// createLogsProcessor creates a logs processor based on this config.
func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs) (processor.Logs, error) {

	oCfg := cfg.(*Config)
	gap := createGroupByAttrsProcessor(set.Logger, oCfg.GroupByKeys)

	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		gap.processLogs,
		processorhelper.WithCapabilities(consumerCapabilities))
}

// createMetricsProcessor creates a metrics processor based on this config.
func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics) (processor.Metrics, error) {

	oCfg := cfg.(*Config)
	gap := createGroupByAttrsProcessor(set.Logger, oCfg.GroupByKeys)

	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		gap.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities))
}
