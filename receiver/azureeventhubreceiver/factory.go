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
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"
)

const (
	// The value of "type" key in configuration.
	typeStr = "azureeventhub"
	// The stability level of the exporter.
	stability = component.StabilityLevelAlpha
)

// NewFactory creates a factory for the Azure Event Hub receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, stability))
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createLogsReceiver(_ context.Context, settings receiver.CreateSettings, cfg component.Config, logs consumer.Logs) (receiver.Logs, error) {

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             settings.ID,
		Transport:              "azureeventhub",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	var converter eventConverter
	switch logFormat(cfg.(*Config).Format) {
	case azureLogFormat:
		converter = newAzureLogFormatConverter(settings)
	case rawLogFormat:
		converter = newRawConverter(settings)
	default:
		converter = newAzureLogFormatConverter(settings)
	}

	return &client{
		settings: settings,
		consumer: logs,
		config:   cfg.(*Config),
		obsrecv:  obsrecv,
		convert:  converter,
	}, nil
}
