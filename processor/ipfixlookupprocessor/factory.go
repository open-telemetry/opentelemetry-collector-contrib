// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipfixlookupprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	// this is the name used to refer to the processor in the config.yaml
	typeStr = "ipfixLookup"
)

func NewFactory() processor.Factory {

	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesToTracesProcessor, component.StabilityLevelAlpha))
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesToTracesProcessor(_ context.Context, params processor.CreateSettings, cfg component.Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	c := newProcessor(params.Logger, cfg)
	c.tracesConsumer = nextConsumer
	return c, nil
}
