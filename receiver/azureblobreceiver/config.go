// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

type Config struct {
	// Azure Blob Storage connection key,
	// which can be found in the Azure Blob Storage resource on the Azure Portal. (no default)
	ConnectionString string `mapstructure:"connection_string"`
	// Configurations of Azure Event Hub triggering on the `Blob Create` event
	EventHub EventHubConfig `mapstructure:"event_hub"`
	// Logs related configurations
	Logs LogsConfig `mapstructure:"logs"`
	// Traces related configurations
	Traces TracesConfig `mapstructure:"traces"`
}

type EventHubConfig struct {
	// Azure Event Hub endpoint triggering on the `Blob Create` event
	EndPoint string `mapstructure:"endpoint"`
}

type LogsConfig struct {
	// Name of the blob container with the logs (default = "logs")
	ContainerName string `mapstructure:"container_name"`
}

type TracesConfig struct {
	// Name of the blob container with the traces (default = "traces")
	ContainerName string `mapstructure:"container_name"`
}
