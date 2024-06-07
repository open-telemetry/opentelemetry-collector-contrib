// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsapplicationsignalsprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsapplicationsignalsprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The stability level of the processor.
	stability = component.StabilityLevelBeta
)

var (
	// The value of "type" key in configuration.
	typeStr, _           = component.NewType("awsapplicationsignals")
	consumerCapabilities = consumer.Capabilities{MutatesData: true}
)

// NewFactory returns a new factory for the aws application signals processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
		processor.WithMetrics(createMetricsProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	ap, err := createProcessor(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		next,
		ap.processTraces,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(ap.StartTraces),
		processorhelper.WithShutdown(ap.Shutdown))
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextMetricsConsumer consumer.Metrics,
) (processor.Metrics, error) {
	ap, err := createProcessor(set, cfg)
	if err != nil {
		return nil, err
	}

	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextMetricsConsumer,
		ap.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities),
		processorhelper.WithStart(ap.StartMetrics),
		processorhelper.WithShutdown(ap.Shutdown))
}

func createProcessor(
	_ processor.Settings,
	_ component.Config,
) (*awsapplicationsignalsprocessor, error) {
	ap := &awsapplicationsignalsprocessor{}

	return ap, nil
}
