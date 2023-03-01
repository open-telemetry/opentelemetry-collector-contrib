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

package routingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const (
	// The value of "type" key in configuration.
	typeStr = "routing"
	// The stability level of the processor.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for the routing processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
		processor.WithMetrics(createMetricsProcessor, stability),
		processor.WithLogs(createLogsProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		AttributeSource: defaultAttributeSource,
		ErrorMode:       ottl.PropagateError,
	}
}

func createTracesProcessor(_ context.Context, params processor.CreateSettings, cfg component.Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	warnIfNotLastInPipeline(nextConsumer, params.Logger)
	return newTracesProcessor(params.TelemetrySettings, cfg), nil
}

func createMetricsProcessor(_ context.Context, params processor.CreateSettings, cfg component.Config, nextConsumer consumer.Metrics) (processor.Metrics, error) {
	warnIfNotLastInPipeline(nextConsumer, params.Logger)
	return newMetricProcessor(params.TelemetrySettings, cfg), nil
}

func createLogsProcessor(_ context.Context, params processor.CreateSettings, cfg component.Config, nextConsumer consumer.Logs) (processor.Logs, error) {
	warnIfNotLastInPipeline(nextConsumer, params.Logger)
	return newLogProcessor(params.TelemetrySettings, cfg), nil
}

func warnIfNotLastInPipeline(nextConsumer interface{}, logger *zap.Logger) {
	_, ok := nextConsumer.(component.Component)
	if ok {
		logger.Warn("another processor has been defined after the routing processor: it will NOT receive any data!")
	}
}
