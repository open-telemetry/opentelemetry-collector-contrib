// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

type TelemetryConfig struct {
	Logs    string `mapstructure:"logs"`
	Metrics string `mapstructure:"metrics"`
	Traces  string `mapstructure:"traces"`
}

type Container TelemetryConfig
type BlobName TelemetryConfig

type BlobNameFormat struct {
	FormatType string    `mapstructure:"format"`
	BlobName   *BlobName `mapstructure:"blob_name"`
	Year       string    `mapstructure:"year"`
	Month      string    `mapstructure:"month"`
	Day        string    `mapstructure:"day"`
	Hour       string    `mapstructure:"hour"`
	Minute     string    `mapstructure:"minute"`
	Second     string    `mapstructure:"second"`
}

type Authentication struct {
	// Type is the authentication type. supported values are connection_string, service_principal, system_managed_identity and user_managed_identity
	Type AuthType `mapstructure:"type"`

	// TenantId is the tenand id for the AAD App. It's only needed when type is service principal.
	TenantId string `mapstructure:"tenant_id"`

	// ClientId is the AAD Application client id. It's needed when type is service principal or user managed identity
	ClientId string `mapstructure:"client_id"`
	// ClientSecret only needed when auth type is service_principal

	ClientSecret string `mapstructure:"client_secret"`

	// ConnectionString to the endpoint.
	ConnectionString string `mapstructure:"connection_string"`
}

type AuthType string

const (
	ConnectionString      AuthType = "connection_string"
	SystemManagedIdentity AuthType = "system_managed_identity"
	UserManagedIdentity   AuthType = "user_managed_identity"
	ServicePrincipal      AuthType = "service_principal"
)

// Config contains the main configuration options for the azure storage blob exporter
type Config struct {
	Url            string          `mapstructure:"url"`
	Container      *Container      `mapstructure:"container"`
	Auth           *Authentication `mapstructure:"auth"`
	BlobNameFormat *BlobNameFormat `mapstructure:"blob_name_format"`
	FormatType     string          `mapstructure:"format"`
	// Encoding to apply. If present, overrides the marshaler configuration option.
	Encoding *component.ID `mapstructure:"encoding"`
}

func (c *Config) Validate() error {
	if c.Url == "" && c.Auth.Type != ConnectionString {
		return errors.New("url cannot be empty when auth type is not connection_string")
	}

	if c.Auth.Type == ConnectionString {
		if c.Auth.ConnectionString == "" {
			return errors.New("connection_string cannot be empty when auth type is connection_string")
		}
	} else if c.Auth.Type == ServicePrincipal {
		if c.Auth.TenantId == "" || c.Auth.ClientId == "" || c.Auth.ClientSecret == "" {
			return errors.New("tenant_id, client_id and client_secret cannot be empty when auth type is service-principal")
		}
	} else if c.Auth.Type == UserManagedIdentity {
		if c.Auth.ClientId == "" {
			return errors.New("client_id cannot be empty when auth type is user_managed_identity")
		}
	}

	if c.FormatType != formatTypeJSON && c.FormatType != formatTypeProto {
		return errors.New("unknown format type: " + c.FormatType)
	}

	return nil
}
