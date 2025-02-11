// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstarttimeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

type metricStartTimeProcessor struct {
	settings processor.Settings
	config   *Config
}

func newMetricStartTimeProcessor(ctx context.Context, set processor.Settings, nextConsumer consumer.Metrics, cfg *Config) (processor.Metrics, error) {
	msp := &metricStartTimeProcessor{
		settings: set,
		config:   cfg,
	}
	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		msp.processMetrics,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

func (msp *metricStartTimeProcessor) processMetrics(_ context.Context, metricData pmetric.Metrics) (pmetric.Metrics, error) {
	// TODO: implement
	return metricData, nil
}
