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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
)

const (
	// The value of "type" key in configuration.
	typeStr = "azureeventhub"
	// The stability level of the exporter.
	stability = component.StabilityLevelAlpha
)

// NewFactory creates a factory for the Azure Event Hub receiver.
func NewFactory() component.ReceiverFactory {
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithLogsReceiver(createLogsReceiver, stability))
}

func createDefaultConfig() component.Config {
	return &Config{ReceiverSettings: config.NewReceiverSettings(component.NewID(typeStr))}
}

func createLogsReceiver(_ context.Context, settings component.ReceiverCreateSettings, receiver component.Config, logs consumer.Logs) (component.LogsReceiver, error) {

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             receiver.ID(),
		Transport:              "azureeventhub",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	var converter eventConverter
	switch logFormat(receiver.(*Config).Format) {
	case azureLogFormat:
		converter = newAzureLogFormatConverter(settings)
	case rawLogFormat:
		converter = newRawConverter(settings)
	default:
		converter = newRawConverter(settings)
	}

	return &client{
		logger:   settings.Logger,
		consumer: logs,
		config:   receiver.(*Config),
		obsrecv:  obsrecv,
		convert:  converter,
	}, nil
}
