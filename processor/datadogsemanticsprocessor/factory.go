// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogsemanticsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor"

import (
	"context"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor/internal/metadata"
)

var consumerCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Datadog semantics processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

type tracesProcessor struct {
	overrideIncomingDatadogFields bool
	attrsTranslator               *attributes.Translator
}

func newTracesProcessor(
	set processor.Settings,
	cfg *Config,
) (*tracesProcessor, error) {
	attrsTranslator, err := attributes.NewTranslator(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	return &tracesProcessor{
		overrideIncomingDatadogFields: cfg.OverrideIncomingDatadogFields,
		attrsTranslator:               attrsTranslator,
	}, nil
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
	tp, err := newTracesProcessor(set, oCfg)
	if err != nil {
		return nil, err
	}
	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		next,
		tp.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities),
	)
}
