// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2"
	"go.opentelemetry.io/collector/component"
)

type logFormat string

const (
	defaultLogFormat logFormat = ""
	rawLogFormat     logFormat = "raw"
	azureLogFormat   logFormat = "azure"
)

var (
	validFormats         = []logFormat{defaultLogFormat, rawLogFormat, azureLogFormat}
	errMissingConnection = errors.New("missing connection")
)

type Config struct {
	Connection               string         `mapstructure:"connection"`
	EventHub                 EventHubConfig `mapstructure:"event_hub"`
	Partition                string         `mapstructure:"partition"`
	Offset                   string         `mapstructure:"offset"`
	StorageID                *component.ID  `mapstructure:"storage"`
	Auth                     *component.ID  `mapstructure:"auth"`
	Format                   string         `mapstructure:"format"`
	ConsumerGroup            string         `mapstructure:"group"`
	ApplySemanticConventions bool           `mapstructure:"apply_semantic_conventions"`
	TimeFormats              TimeFormat     `mapstructure:"time_formats"`
	MetricAggregation        string         `mapstructure:"metric_aggregation"`

	// BlobCheckpointStore enables distributed consumption using Azure Blob Storage
	// for checkpoint coordination. When configured, the receiver uses the Azure SDK
	// Processor for dynamic partition assignment across multiple collector instances.
	BlobCheckpointStore *BlobCheckpointStoreConfig `mapstructure:"blob_checkpoint_store"`

	// azeventhub lib specific
	PollRate      int `mapstructure:"poll_rate"`
	MaxPollEvents int `mapstructure:"max_poll_events"`
}

// BlobCheckpointStoreConfig defines the configuration for Azure Blob Storage
// based checkpoint coordination used in distributed consumption mode.
type BlobCheckpointStoreConfig struct {
	// Connection is the connection string for Azure Blob Storage.
	// Required when the parent config does not use auth.
	Connection string `mapstructure:"connection"`
	// StorageAccountURL is the blob service URL (e.g., https://myaccount.blob.core.windows.net).
	// Required when the parent config uses auth.
	StorageAccountURL string `mapstructure:"storage_account_url"`
	// ContainerName is the blob container used for checkpoint data. Required.
	ContainerName string `mapstructure:"container_name"`
}

// EventHubConfig defines the configuration for an Azure Event Hub when
// using authentication.
type EventHubConfig struct {
	// Name is the name of the Event Hub.
	Name string `mapstructure:"name"`
	// Namespace is the fully qualified namespace of the Event Hub.
	Namespace string `mapstructure:"namespace"`
}

type TimeFormat struct {
	Logs    []string `mapstructure:"logs"`
	Metrics []string `mapstructure:"metrics"`
	Traces  []string `mapstructure:"traces"`
}

// Validate config
func (config *Config) Validate() error {
	if config.Auth != nil {
		if config.EventHub.Name == "" {
			return errors.New("event_hub.name is required when using auth")
		}
		if config.EventHub.Namespace == "" {
			return errors.New("event_hub.namespace is required when using auth")
		}
	} else {
		if config.Connection == "" {
			return errMissingConnection
		}
		if _, err := azeventhubs.ParseConnectionString(config.Connection); err != nil {
			return err
		}
	}

	switch logFormat(config.Format) {
	case defaultLogFormat, rawLogFormat, azureLogFormat: // valid
	default:
		return fmt.Errorf("invalid format; must be one of %#v", validFormats)
	}

	if config.Partition == "" && config.Offset != "" {
		return errors.New("cannot use 'offset' without 'partition'")
	}

	if config.BlobCheckpointStore != nil {
		if config.BlobCheckpointStore.ContainerName == "" {
			return errors.New("blob_checkpoint_store.container_name is required")
		}
		if config.Auth == nil && config.BlobCheckpointStore.Connection == "" {
			return errors.New("blob_checkpoint_store.connection is required when not using auth")
		}
		if config.Auth != nil && config.BlobCheckpointStore.StorageAccountURL == "" {
			return errors.New("blob_checkpoint_store.storage_account_url is required when using auth")
		}
		if config.Partition != "" || config.Offset != "" {
			return errors.New("blob_checkpoint_store is mutually exclusive with partition and offset")
		}
		if config.StorageID != nil {
			return errors.New("blob_checkpoint_store is mutually exclusive with storage")
		}
	}

	return nil
}
