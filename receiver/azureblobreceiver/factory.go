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

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
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

type blobReceiverFactory struct {
	receivers *sharedcomponent.SharedComponents
}

// NewFactory returns a factory for Azure Blob receiver.
func NewFactory() receiver.Factory {
	f := &blobReceiverFactory{
		receivers: sharedcomponent.NewSharedComponents(),
	}

	return receiver.NewFactory(
		typeStr,
		f.createDefaultConfig,
		receiver.WithTraces(f.createTracesReceiver, component.StabilityLevelBeta),
		receiver.WithLogs(f.createLogsReceiver, component.StabilityLevelBeta))
}

func (f *blobReceiverFactory) createDefaultConfig() component.Config {
	return &Config{
		Logs:   LogsConfig{ContainerName: logsContainerName},
		Traces: TracesConfig{ContainerName: tracesContainerName},
	}
}

func (f *blobReceiverFactory) createLogsReceiver(
	ctx context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {

	receiver, err := f.getReceiver(set, cfg)

	if err != nil {
		set.Logger.Error(err.Error())
		return nil, err
	}

	receiver.(logsDataConsumer).setNextLogsConsumer(nextConsumer)

	return receiver, nil
}

func (f *blobReceiverFactory) createTracesReceiver(
	ctx context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {

	receiver, err := f.getReceiver(set, cfg)

	if err != nil {
		set.Logger.Error(err.Error())
		return nil, err
	}

	receiver.(tracesDataConsumer).setNextTracesConsumer(nextConsumer)
	return receiver, nil
}

func (f *blobReceiverFactory) getReceiver(
	set receiver.CreateSettings,
	cfg component.Config) (component.Component, error) {

	var err error
	r := f.receivers.GetOrAdd(cfg, func() component.Component {
		receiverConfig, ok := cfg.(*Config)

		if !ok {
			err = errUnexpectedConfigurationType
			return nil
		}

		var beh blobEventHandler
		beh, err = f.getBlobEventHandler(receiverConfig, set.Logger)
		if err != nil {
			return nil
		}

		var receiver component.Component
		receiver, err = newReceiver(set, beh)
		return receiver
	})

	if err != nil {
		return nil, err
	}

	return r.Unwrap(), err
}

func (f *blobReceiverFactory) getBlobEventHandler(cfg *Config, logger *zap.Logger) (blobEventHandler, error) {
	bc, err := newBlobClient(cfg.ConnectionString, logger)
	if err != nil {
		return nil, err
	}

	return newBlobEventHandler(cfg.EventHub.EndPoint, cfg.Logs.ContainerName, cfg.Traces.ContainerName, bc, logger),
		nil
}
