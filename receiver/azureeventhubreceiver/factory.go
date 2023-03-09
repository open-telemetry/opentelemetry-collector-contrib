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

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"context"
	"errors"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	// The value of "type" key in configuration.
	typeStr = "azureeventhub"

	// The stability level of the exporter.
	stability = component.StabilityLevelAlpha

	// The receiver scope name
	receiverScopeName = "otelcol/" + typeStr
)

var (
	errUnexpectedConfigurationType = errors.New("Failed to cast configuration to Azure Event Hub Config")
)

type eventhubReceiverFactory struct {
	receivers *sharedcomponent.SharedComponents
}

// NewFactory creates a factory for the Azure Event Hub receiver.
func NewFactory() receiver.Factory {
	f := &eventhubReceiverFactory{
		receivers: sharedcomponent.NewSharedComponents(),
	}

	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithLogs(f.createLogsReceiver, stability),
		receiver.WithMetrics(f.createMetricsReceiver, stability))
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func (f *eventhubReceiverFactory) createLogsReceiver(
	ctx context.Context,
	settings receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {

	receiverConfig := cfg.(*Config)
	receiver, err := newReceiver(component.DataTypeLogs, receiverConfig, settings)

	if err != nil {
		return nil, err
	}

	receiver.(dataConsumer).setNextLogsConsumer(nextConsumer)

	return receiver, nil
}

func (f *eventhubReceiverFactory) createMetricsReceiver(
	ctx context.Context,
	settings receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {

	receiverConfig := cfg.(*Config)
	receiver, err := newReceiver(component.DataTypeMetrics, receiverConfig, settings)

	if err != nil {
		return nil, err
	}

	receiver.(dataConsumer).setNextMetricsConsumer(nextConsumer)

	return receiver, nil
}
