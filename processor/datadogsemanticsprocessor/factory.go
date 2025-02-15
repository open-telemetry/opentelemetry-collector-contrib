// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogsemanticsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor"

import (
	"context"

	"github.com/DataDog/datadog-agent/pkg/trace/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor/internal/metadata"
)

var consumerCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the k8s processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

type tracesProcessor struct {
	agentCfg                      *config.AgentConfig
	overrideIncomingDatadogFields bool
}

func newTracesProcessor(cfg *Config) *tracesProcessor {
	return &tracesProcessor{
		agentCfg: &config.AgentConfig{
			OTLPReceiver: &config.OTLP{SpanNameAsResourceName: false, SpanNameRemappings: nil},
			Features: map[string]struct{}{
				"enable_otlp_compute_top_level_by_span_kind":  {},
				"enable_operation_and_resource_name_logic_v2": {},
			},
		},
		overrideIncomingDatadogFields: cfg.OverrideIncomingDatadogFields,
	}
}

func createDefaultConfig() component.Config {
	return &Config{
		OverrideIncomingDatadogFields: false,
	}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)
	tp := newTracesProcessor(oCfg)
	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		next,
		tp.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities),
	)
}
