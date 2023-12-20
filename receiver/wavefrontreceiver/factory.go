// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package wavefrontreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver/internal/metadata"
)

const (
	// tcpIdleTimeoutDefault is the default timeout for idle TCP connections.
	tcpIdleTimeoutDefault = 30 * time.Second
)

// This file implements factory for the Wavefront receiver.

// NewFactory creates a factory for WaveFront receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: "localhost:2003",
		},
		TCPIdleTimeout: tcpIdleTimeoutDefault,
	}
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	rCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("a wavefront receiver config was expected by the receiver factory, but got %T", rCfg)
	}
	return newMetricsReceiver(rCfg, params, consumer), nil
}
