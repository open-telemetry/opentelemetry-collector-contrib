// Copyright The OpenTelemetry Authors
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

package mockawsxrayreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// This file implements factory for awsxray receiver.

const (
	// The value of "type" key in configuration.
	typeStr = "mock_receiver"

	// Default endpoints to bind to.
	defaultEndpoint = ":7276"
)

// NewFactory creates a factory for SAPM receiver.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		receiverhelper.WithTraces(createTracesReceiver))
}

// CreateDefaultConfig creates the default configuration for Jaeger receiver.
func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewID(typeStr)),
		Endpoint:         defaultEndpoint,
	}
}

// CreateTracesReceiver creates a trace receiver based on provided config.
func createTracesReceiver(
	ctx context.Context,
	params component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Traces,
) (component.TracesReceiver, error) {
	rCfg := cfg.(*Config)
	return New(nextConsumer, params, rCfg)
}
