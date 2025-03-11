// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/multierr"
)

var (
	// Predefined error responses for configuration validation failures
	errMissingTenantID          = errors.New(`"TenantID" is not specified in config`)
	errMissingClientID          = errors.New(`"ClientID" is not specified in config`)
	errMissingClientSecret      = errors.New(`"ClientSecret" is not specified in config`)
	errMissingStorageAccountURL = errors.New(`"StorageAccountURL" is not specified in config`)
	errMissingConnectionString  = errors.New(`"ConnectionString" is not specified in config`)
)

type Config struct {
	// Type of authentication to use
	Authentication AuthType `mapstructure:"auth"`
	// Azure Blob Storage connection key,
	// which can be found in the Azure Blob Storage resource on the Azure Portal. (no default)
	ConnectionString string `mapstructure:"connection_string"`
	// Storage Account URL, used with Service Principal authentication
	StorageAccountURL string `mapstructure:"storage_account_url"`
	// Configuration for the Service Principal credentials
	ServicePrincipal ServicePrincipalConfig `mapstructure:"service_principal"`
	// Azure Cloud to authenticate against, used with Service Principal authentication
	Cloud CloudType `mapstructure:"cloud"`
	// Configurations of Azure Event Hub triggering on the `Blob Create` event
	EventHub EventHubConfig `mapstructure:"event_hub"`
	// Logs related configurations
	Logs LogsConfig `mapstructure:"logs"`
	// Traces related configurations
	Traces TracesConfig `mapstructure:"traces"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type EventHubConfig struct {
	// Azure Event Hub endpoint triggering on the `Blob Create` event
	EndPoint string `mapstructure:"endpoint"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type LogsConfig struct {
	// Name of the blob container with the logs (default = "logs")
	ContainerName string `mapstructure:"container_name"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type TracesConfig struct {
	// Name of the blob container with the traces (default = "traces")
	ContainerName string `mapstructure:"container_name"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type ServicePrincipalConfig struct {
	// Tenant ID, used with Service Principal authentication
	TenantID string `mapstructure:"tenant_id"`
	// Client ID, used with Service Principal authentication
	ClientID string `mapstructure:"client_id"`
	// Client secret, used with Service Principal authentication
	ClientSecret configopaque.String `mapstructure:"client_secret"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type AuthType string

const (
	ServicePrincipalAuth AuthType = "service_principal"
	ConnectionStringAuth AuthType = "connection_string"
	DefaultAuth          AuthType = "default"
)

func (e *AuthType) UnmarshalText(text []byte) error {
	str := AuthType(text)
	switch str {
	case ServicePrincipalAuth, ConnectionStringAuth, DefaultAuth:
		*e = str
		return nil
	default:
		return fmt.Errorf("authentication %v is not supported. supported authentications include [%v,%v,%v]", str, ServicePrincipalAuth, ConnectionStringAuth, DefaultAuth)
	}
}

type CloudType string

const (
	AzureCloudType           = "AzureCloud"
	AzureGovernmentCloudType = "AzureUSGovernment"
)

func (e *CloudType) UnmarshalText(text []byte) error {
	str := CloudType(text)
	switch str {
	case AzureCloudType, AzureGovernmentCloudType:
		*e = str
		return nil
	default:
		return fmt.Errorf("cloud %v is not supported. supported options include [%v,%v]", str, AzureCloudType, AzureGovernmentCloudType)
	}
}

// Validate validates the configuration by checking for missing or invalid fields
func (c Config) Validate() (err error) {
	if c.Authentication == ServicePrincipalAuth {
		if c.ServicePrincipal.TenantID == "" {
			err = multierr.Append(err, errMissingTenantID)
		}

		if c.ServicePrincipal.ClientID == "" {
			err = multierr.Append(err, errMissingClientID)
		}

		if c.ServicePrincipal.ClientSecret == "" {
			err = multierr.Append(err, errMissingClientSecret)
		}

		if c.StorageAccountURL == "" {
			err = multierr.Append(err, errMissingStorageAccountURL)
		}
	} else if c.Authentication == ConnectionStringAuth {
		if c.ConnectionString == "" {
			err = multierr.Append(err, errMissingConnectionString)
		}
	}

	return
}
