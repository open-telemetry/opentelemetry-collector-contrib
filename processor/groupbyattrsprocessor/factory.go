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

package groupbyattrsprocessor

import (
	"context"
	"fmt"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

const (
	// typeStr is the value of "type" for this processor in the configuration.
	typeStr configmodels.Type = "groupbyattrs"
)

var (
	errAtLeastOneAttributeNeeded = fmt.Errorf("option 'groupByKeys' must include at least one non-empty attribute name")
	processorCapabilities        = component.ProcessorCapabilities{MutatesConsumedData: true}
)

// NewFactory returns a new factory for the Filter processor.
func NewFactory() component.ProcessorFactory {
	// TODO: as with other -contrib factories registering metrics, this is causing the error being ignored
	_ = view.Register(MetricViews()...)

	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithTraces(createTraceProcessor),
		processorhelper.WithLogs(createLogsProcessor))
}

// createDefaultConfig creates the default configuration for the processor.
func createDefaultConfig() configmodels.Processor {
	return &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: typeStr,
			NameVal: string(typeStr),
		},
		GroupByKeys: []string{},
	}
}

func createGroupByAttrsProcessor(logger *zap.Logger, attributes []string) (*groupByAttrsProcessor, error) {
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

	if len(nonEmptyAttributes) == 0 {
		return nil, errAtLeastOneAttributeNeeded
	}

	return &groupByAttrsProcessor{logger: logger, groupByKeys: nonEmptyAttributes}, nil
}

// createTraceProcessor creates a trace processor based on this config.
func createTraceProcessor(
	_ context.Context,
	params component.ProcessorCreateParams,
	cfg configmodels.Processor,
	nextConsumer consumer.TracesConsumer) (component.TracesProcessor, error) {

	oCfg := cfg.(*Config)
	gap, err := createGroupByAttrsProcessor(params.Logger, oCfg.GroupByKeys)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTraceProcessor(
		cfg,
		nextConsumer,
		gap,
		processorhelper.WithCapabilities(processorCapabilities))
}

// createLogsProcessor creates a metrics processor based on this config.
func createLogsProcessor(
	_ context.Context,
	params component.ProcessorCreateParams,
	cfg configmodels.Processor,
	nextConsumer consumer.LogsConsumer) (component.LogsProcessor, error) {

	oCfg := cfg.(*Config)
	gap, err := createGroupByAttrsProcessor(params.Logger, oCfg.GroupByKeys)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewLogsProcessor(
		cfg,
		nextConsumer,
		gap,
		processorhelper.WithCapabilities(processorCapabilities))
}
