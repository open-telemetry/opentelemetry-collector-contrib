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

package mockawsxrayreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatareceivers/mockawsxrayreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

// This file implements factory for awsxray receiver.

const (
	// The value of "type" key in configuration.
	typeStr = "mock_receiver"
	// stability level of test component
	stability = component.StabilityLevelDevelopment

	// Default endpoints to bind to.
	defaultEndpoint = ":7276"
)

// NewFactory creates a factory for SAPM receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, stability))
}

// CreateDefaultConfig creates the default configuration for Jaeger receiver.
func createDefaultConfig() component.Config {
	return &Config{
		Endpoint: defaultEndpoint,
	}
}

// CreateTracesReceiver creates a trace receiver based on provided config.
func createTracesReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	rCfg := cfg.(*Config)
	return New(nextConsumer, params, rCfg)
}
