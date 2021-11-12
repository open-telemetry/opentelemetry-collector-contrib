// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wavefrontreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/transport"
)

// This file implements factory for the Wavefront receiver.

const (
	// The value of "type" key in configuration.
	typeStr = "wavefront"
)

// NewFactory creates a factory for WaveFront receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		TCPAddr: confignet.TCPAddr{
			Endpoint: "localhost:2003",
		},
		TCPIdleTimeout: transport.TCPIdleTimeoutDefault,
	}
}

func createMetricsReceiver(
	ctx context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	consumer consumer.Metrics,
) (component.MetricsReceiver, error) {

	rCfg := cfg.(*Config)

	// Wavefront is very similar to Carbon: it is TCP based in which each received
	// text line represents a single metric data point. They differ on the format
	// of their textual representation.
	//
	// The Wavefront receiver leverages the Carbon receiver code by implementing
	// a dedicated parser for its format.
	carbonCfg := carbonreceiver.Config{
		ReceiverSettings: rCfg.ReceiverSettings,
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
