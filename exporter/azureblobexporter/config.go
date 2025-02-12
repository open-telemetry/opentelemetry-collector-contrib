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

type (
	Container TelemetryConfig
	BlobName  TelemetryConfig
)

type BlobNameFormat struct {
	MetricsFormat  string            `mapstructure:"metrics_format"`
	LogsFormat     string            `mapstructure:"logs_format"`
	TracesFormat   string            `mapstructure:"traces_format"`
	SerialNumRange int64             `mapstructure:"serial_num_range"`
	Params         map[string]string `mapstructure:"params"`
}

type Authentication struct {
	// Type is the authentication type. supported values are connection_string, service_principal, system_managed_identity and user_managed_identity
	Type AuthType `mapstructure:"type"`

	// TenantID is the tenand id for the AAD App. It's only needed when type is service principal.
	TenantID string `mapstructure:"tenant_id"`

	// ClientID is the AAD Application client id. It's needed when type is service principal or user managed identity
	ClientID string `mapstructure:"client_id"`
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
	Endpoint       string          `mapstructure:"endpoint"`
	Container      *Container      `mapstructure:"container"`
	Auth           *Authentication `mapstructure:"auth"`
	BlobNameFormat *BlobNameFormat `mapstructure:"blob_name_format"`
	FormatType     string          `mapstructure:"format"`
	// Encoding to apply. If present, overrides the marshaler configuration option.
	Encoding *component.ID `mapstructure:"encoding"`
}

func (c *Config) Validate() error {
	if c.Endpoint == "" && c.Auth.Type != ConnectionString {
		return errors.New("endpoint cannot be empty when auth type is not connection_string")
	}

	switch c.Auth.Type {
	case ConnectionString:
		if c.Auth.ConnectionString == "" {
			return errors.New("connection_string cannot be empty when auth type is connection_string")
		}
	case ServicePrincipal:
		if c.Auth.TenantID == "" || c.Auth.ClientID == "" || c.Auth.ClientSecret == "" {
			return errors.New("tenant_id, client_id and client_secret cannot be empty when auth type is service-principal")
		}
	case UserManagedIdentity:
		if c.Auth.ClientID == "" {
			return errors.New("client_id cannot be empty when auth type is user_managed_identity")
		}
	}

	if c.FormatType != formatTypeJSON && c.FormatType != formatTypeProto {
		return errors.New("unknown format type: " + c.FormatType)
	}

	return nil
}
