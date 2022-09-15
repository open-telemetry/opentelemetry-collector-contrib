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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

const (
	// The value of "type" key in configuration.
	typeStr = "logstransform"
	// The stability level of the processor.
	stability = component.StabilityLevelInDevelopment
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Logs Transform processor.
func NewFactory() component.ProcessorFactory {
	return component.NewProcessorFactory(
		typeStr,
		createDefaultConfig,
		component.WithLogsProcessor(createLogsProcessor, stability))
}

// Note: This isn't a valid configuration because the processor would do no work.
func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		BaseConfig: adapter.BaseConfig{
			Operators: adapter.OperatorConfigs{},
			Converter: adapter.ConverterConfig{
				MaxFlushCount: 100,
				FlushInterval: 100 * time.Millisecond,
			},
		},
	}
}

func createLogsProcessor(
	ctx context.Context,
	set component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Logs) (component.LogsProcessor, error) {
	pCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("could not initialize logs transform processor")
	}

	operators, err := pCfg.BaseConfig.DecodeOperatorConfigs()
	if err != nil {
		return nil, err
	}
	if len(operators) == 0 {
		return nil, errors.New("no operators were configured for this logs transform processor")
	}

	proc := &logsTransformProcessor{
		id:     cfg.ID(),
		logger: set.Logger,
		config: pCfg,
	}
	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.processLogs,
		processorhelper.WithStart(proc.Start),
		processorhelper.WithShutdown(proc.Shutdown),
		processorhelper.WithCapabilities(processorCapabilities))
}
