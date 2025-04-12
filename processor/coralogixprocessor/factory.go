// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/metadata"
)

// NewFactory returns a new factory for the Span processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, component.StabilityLevelDevelopment))
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesProcessor(
	ctx context.Context,
	params processor.Settings,
	baseCfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	coralogixCfg := baseCfg.(*Config)

	coralogixProcessor, err := newCoralogixProcessor(ctx,
		params,
		coralogixCfg,
		nextConsumer)
	if err != nil {
		return nil, err
	}

	return coralogixProcessor, nil
}
