// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package wavefrontreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/transport"
)

// This file implements factory for the Wavefront receiver.

const (
	// The value of "type" key in configuration.
	typeStr = "wavefront"
	// The stability level of the receiver.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for WaveFront receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
}

func createDefaultConfig() component.Config {
	return &Config{
		TCPAddr: confignet.TCPAddr{
			Endpoint: "localhost:2003",
		},
		TCPIdleTimeout: transport.TCPIdleTimeoutDefault,
	}
}

func createMetricsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {

	rCfg := cfg.(*Config)

	// Wavefront is very similar to Carbon: it is TCP based in which each received
	// text line represents a single metric data point. They differ on the format
	// of their textual representation.
	//
	// The Wavefront receiver leverages the Carbon receiver code by implementing
	// a dedicated parser for its format.
	carbonCfg := carbonreceiver.Config{
		NetAddr: confignet.NetAddr{
			Endpoint:  rCfg.Endpoint,
			Transport: "tcp",
		},
		TCPIdleTimeout: rCfg.TCPIdleTimeout,
		Parser: &protocol.Config{
			Type: "plaintext", // TODO: update after other parsers are implemented for Carbon receiver.
			Config: &WavefrontParser{
				ExtractCollectdTags: rCfg.ExtractCollectdTags,
			},
		},
	}
	return carbonreceiver.New(params, carbonCfg, consumer)
}
