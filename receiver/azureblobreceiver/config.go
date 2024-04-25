// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"errors"
	"fmt"

	"go.uber.org/multierr"
)

const (
	azureCloud           = "AzureCloud"
	azureGovernmentCloud = "AzureUSGovernment"
)

var (
	// Predefined error responses for configuration validation failures
	errMissingTenantID          = errors.New(`"TenantID" is not specified in config`)
	errMissingClientID          = errors.New(`"ClientID" is not specified in config`)
	errMissingClientSecret      = errors.New(`"ClientSecret" is not specified in config`)
	errMissingStorageAccountURL = errors.New(`"StorageAccountURL" is not specified in config`)
	errMissingConnectionString  = errors.New(`"ConnectionString" is not specified in config`)
	errInvalidCloud             = errors.New(`"Cloud" is invalid`)
)

type Config struct {
	// Type of authentication to use
	Authentication string `mapstructure:"auth"`
	// Azure Blob Storage connection key,
	// which can be found in the Azure Blob Storage resource on the Azure Portal. (no default)
	ConnectionString string `mapstructure:"connection_string"`
	// Storage Account URL, used with Service Principal authentication
	StorageAccountURL string `mapstructure:"storage_account_url"`
	// Tenant ID, used with Service Principal authentication
	TenantID string `mapstructure:"tenant_id"`
	// Client ID, used with Service Principal authentication
	ClientID string `mapstructure:"client_id"`
	// Client secret, used with Service Principal authentication
	ClientSecret string `mapstructure:"client_secret"`
	// Azure Cloud to authenticate against, used with Service Principal authentication
	Cloud string `mapstructure:"cloud"`
	// Configurations of Azure Event Hub triggering on the `Blob Create` event
	EventHub EventHubConfig `mapstructure:"event_hub"`
	// Logs related configurations
	Logs LogsConfig `mapstructure:"logs"`
	// Traces related configurations
	Traces TracesConfig `mapstructure:"traces"`
}

const (
	servicePrincipal = "service_principal"
	connectionString = "connection_string"
)

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

// Validate validates the configuration by checking for missing or invalid fields
func (c Config) Validate() (err error) {
	switch c.Authentication {
	case servicePrincipal:
		if c.TenantID == "" {
			err = multierr.Append(err, errMissingTenantID)
		}

		if c.ClientID == "" {
			err = multierr.Append(err, errMissingClientID)
		}

		if c.ClientSecret == "" {
			err = multierr.Append(err, errMissingClientSecret)
		}

		if c.StorageAccountURL == "" {
			err = multierr.Append(err, errMissingStorageAccountURL)
		}

		if c.Cloud != azureCloud && c.Cloud != azureGovernmentCloud {
			err = multierr.Append(err, errInvalidCloud)
		}
	case connectionString:
		if c.ConnectionString == "" {
			err = multierr.Append(err, errMissingConnectionString)
		}
	default:
		return fmt.Errorf("authentication %v is not supported. supported authentications include [%v,%v]", c.Authentication, servicePrincipal, connectionString)
	}

	return
}
