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

package filterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr = "filter"
	// The stability level of the processor.
	stability = component.StabilityLevelAlpha
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Filter processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, stability),
		processor.WithLogs(createLogsProcessor, stability),
		processor.WithTraces(createTracesProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	oCfg := cfg.(*Config)
	if oCfg.Metrics.Include != nil || oCfg.Metrics.Exclude != nil {
		set.Logger.Warn(
			"The metric `include` and `exclude` configuration options have been deprecated and will be removed in 0.74.0. Use `metric` instead.",
			zap.Any("include", oCfg.Metrics.Include),
			zap.Any("exclude", oCfg.Metrics.Exclude),
		)
	}
	fp, err := newFilterMetricProcessor(set.TelemetrySettings, oCfg)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		fp.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities))
}

func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)
	if oCfg.Logs.Include != nil || oCfg.Logs.Exclude != nil {
		set.Logger.Warn(
			"The log `include` and `exclude` configuration options have been deprecated and will be removed in 0.74.0. Use `log_record` instead.",
			zap.Any("include", oCfg.Logs.Include),
			zap.Any("exclude", oCfg.Logs.Exclude),
		)
	}
	fp, err := newFilterLogsProcessor(set.TelemetrySettings, oCfg)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		fp.processLogs,
		processorhelper.WithCapabilities(processorCapabilities))
}

func createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)
	if oCfg.Spans.Include != nil || oCfg.Spans.Exclude != nil {
		set.Logger.Warn(
			"The span `include` and `exclude` configuration options have been deprecated and will be removed in 0.74.0. Use `traces.span` instead.",
			zap.Any("include", oCfg.Spans.Include),
			zap.Any("exclude", oCfg.Spans.Exclude),
		)
	}
	fp, err := newFilterSpansProcessor(set.TelemetrySettings, oCfg)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		fp.processTraces,
		processorhelper.WithCapabilities(processorCapabilities))
}
