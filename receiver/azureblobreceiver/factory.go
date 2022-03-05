// Copyright OpenTelemetry Authors
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

package azureblobreceiver

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

const (
	// The value of "type" key in configuration.
	typeStr             = "azureblob"
	logsContainerName   = "logs"
	tracesContainerName = "traces"
)

var (
	errUnexpectedConfigurationType = errors.New("failed to cast configuration to Azure Blob Config")
)

type factory struct {
	blobEventHandler BlobEventHandler
}

// NewFactory returns a factory for Azure Blob receiver.
func NewFactory() component.ReceiverFactory {
	f := &factory{}
	return receiverhelper.NewFactory(
		typeStr,
		f.createDefaultConfig,
		receiverhelper.WithTraces(f.createTracesReceiver),
		receiverhelper.WithLogs(f.createLogsReceiver))
}

func (f *factory) createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings:    config.NewReceiverSettings(config.NewComponentID(typeStr)),
		LogsContainerName:   logsContainerName,
		TracesContainerName: tracesContainerName,
	}
}

// func createTracesExporter(
// 	ctx context.Context,
// 	set component.ExporterCreateSettings,
// 	cfg config.Exporter,
// ) (component.TracesExporter, error) {
// 	exporterConfig, ok := cfg.(*Config)

// 	if !ok {
// 		return nil, errUnexpectedConfigurationType
// 	}

// 	bc, err := NewBlobClient(exporterConfig.ConnectionString, exporterConfig.TracesContainerName, set.Logger)
// 	if err != nil {
// 		set.Logger.Error(err.Error())
// 	}

// 	return newTracesExporter(exporterConfig, bc, set)
// }

func (f *factory) createLogsReceiver(
	ctx context.Context,
	set component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Logs) (component.LogsReceiver, error) {
	receiverConfig, ok := cfg.(*Config)

	if !ok {
		return nil, errUnexpectedConfigurationType
	}

	blobEventHandler, err := f.getBlobEventHandler(receiverConfig, set.Logger)

	if err != nil {
		return nil, err
	}

	return NewLogsReceiver(*receiverConfig, set, nextConsumer, blobEventHandler)
}
func (f *factory) createTracesReceiver(
	ctx context.Context,
	set component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Traces) (component.TracesReceiver, error) {
	receiverConfig, ok := cfg.(*Config)

	if !ok {
		return nil, errUnexpectedConfigurationType
	}

	blobEventHandler, err := f.getBlobEventHandler(receiverConfig, set.Logger)

	if err != nil {
		return nil, err
	}

	return NewTraceReceiver(*receiverConfig, set, nextConsumer, blobEventHandler)
}

func (f *factory) getBlobEventHandler(cfg *Config, logger *zap.Logger) (BlobEventHandler, error) {
	if f.blobEventHandler == nil {
		bc, err := NewBlobClient(cfg.ConnectionString, logger)
		if err != nil {
			return nil, err
		}

		f.blobEventHandler = NewBlobEventHandler(cfg.EventHubEndPoint, cfg.LogsContainerName, cfg.TracesContainerName, bc, logger)

	}

	return f.blobEventHandler, nil
}

// func (f *kafkaReceiverFactory) createLogsReceiver(
// 	_ context.Context,
// 	set component.ReceiverCreateSettings,
// 	cfg config.Receiver,
// 	nextConsumer consumer.Logs,
// ) (component.LogsReceiver, error) {
// 	c := cfg.(*Config)
// 	r, err := newLogsReceiver(*c, set, f.logsUnmarshalers, nextConsumer)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return r, nil
// }
