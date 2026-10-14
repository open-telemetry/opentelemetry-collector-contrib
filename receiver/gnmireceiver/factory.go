// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnmireceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gnmireceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gnmireceiver/internal/metadata"
)

// NewFactory creates a receiver factory for the gNMI receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsReceiver(
	_ context.Context,
	_ receiver.Settings,
	_ component.Config,
	_ consumer.Metrics,
) (receiver.Metrics, error) {
	return &gnmiReceiver{}, nil
}

type gnmiReceiver struct{}

func (*gnmiReceiver) Start(context.Context, component.Host) error { return nil }

func (*gnmiReceiver) Shutdown(context.Context) error { return nil }
