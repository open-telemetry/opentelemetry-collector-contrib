// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
)

const (
	componentType config.Type = "solace"
	// The stability level of the receiver.
	stability = component.StabilityLevelInDevelopment

	// default value for max unaked messages
	defaultMaxUnaked uint32 = 1000
	// default value for host
	defaultHost string = "localhost:5671"
)

// NewFactory creates a factory for Solace receiver.
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		componentType,
		createDefaultConfig,
		component.WithTracesReceiver(createTracesReceiver, stability),
	)
}

// createDefaultConfig creates the default configuration for receiver.
func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(componentType)),
		Broker:           []string{defaultHost},
		MaxUnacked:       defaultMaxUnaked,
		Auth:             Authentication{},
		TLS: configtls.TLSClientSetting{
			InsecureSkipVerify: false,
			Insecure:           false,
		},
		Flow: FlowControl{
			DelayedRetry: &FlowControlDelayedRetry{
				Delay: 10 * time.Millisecond,
			},
		},
	}
}

// CreateTracesReceiver creates a trace receiver based on provided config. Component is not shared
func createTracesReceiver(
	_ context.Context,
	params component.ReceiverCreateSettings,
	receiverConfig config.Receiver,
	nextConsumer consumer.Traces,
) (component.TracesReceiver, error) {
	cfg, ok := receiverConfig.(*Config)
	if !ok {
		return nil, component.ErrDataTypeIsNotSupported
	}
	// pass cfg, params and next consumer through
	return newTracesReceiver(cfg, params, nextConsumer)
}
