// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver/internal/metadata"
)

const (
	logsContainerName   = "logs"
	tracesContainerName = "traces"
	defaultCloud        = AzureCloudType
)

var errUnexpectedConfigurationType = errors.New("failed to cast configuration to Azure Blob Config")

type blobReceiverFactory struct {
	receivers *sharedcomponent.SharedComponents
}

// NewFactory returns a factory for Azure Blob receiver.
func NewFactory() receiver.Factory {
	f := &blobReceiverFactory{
		receivers: sharedcomponent.NewSharedComponents(),
	}

	return receiver.NewFactory(
		metadata.Type,
		f.createDefaultConfig,
		receiver.WithTraces(f.createTracesReceiver, metadata.TracesStability),
		receiver.WithLogs(f.createLogsReceiver, metadata.LogsStability))
}

func (f *blobReceiverFactory) createDefaultConfig() component.Config {
	return &Config{
		Logs:           LogsConfig{ContainerName: logsContainerName},
		Traces:         TracesConfig{ContainerName: tracesContainerName},
		Authentication: ConnectionStringAuth,
		Cloud:          defaultCloud,
	}
}

func (f *blobReceiverFactory) createLogsReceiver(
	_ context.Context,
	set receiver.Settings,
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
	_ context.Context,
	set receiver.Settings,
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
	set receiver.Settings,
	cfg component.Config,
) (component.Component, error) {
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
	var bc blobClient
	var err error

	switch cfg.Authentication {
	case ConnectionStringAuth:
		bc, err = newBlobClientFromConnectionString(cfg.ConnectionString, logger)
		if err != nil {
			return nil, err
		}
	case ServicePrincipalAuth:
		cred, err := azidentity.NewClientSecretCredential(cfg.ServicePrincipal.TenantID, cfg.ServicePrincipal.ClientID, string(cfg.ServicePrincipal.ClientSecret), nil)
		if err != nil {
			return nil, err
		}
		bc, err = newBlobClientFromCredential(cfg.StorageAccountURL, cred, logger)
		if err != nil {
			return nil, err
		}
	case DefaultAuth:
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, err
		}
		bc, err = newBlobClientFromCredential(cfg.StorageAccountURL, cred, logger)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown authentication %v", cfg.Authentication)
	}

	return newBlobEventHandler(cfg.EventHub.EndPoint, cfg.Logs.ContainerName, cfg.Traces.ContainerName, bc, logger),
		nil
}
