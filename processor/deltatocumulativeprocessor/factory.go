// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	ltel "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/lineartelemetry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
	)
}

func createMetricsProcessor(_ context.Context, set processor.Settings, cfg component.Config, next consumer.Metrics) (processor.Metrics, error) {
	pcfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("configuration parsing error")
	}

	ltel, err := ltel.New(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	proc := newProcessor(pcfg, set.Logger, &ltel.TelemetryBuilder, next)
	linear := newLinear(pcfg, ltel, proc)

	return Chain{linear, proc}, nil
}
