// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubsexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
)

type TelemetryConfig struct {
	Logs    string `mapstructure:"logs"`
	Metrics string `mapstructure:"metrics"`
	Traces  string `mapstructure:"traces"`
}

type Encodings struct {
	Logs    *component.ID `mapstructure:"logs"`
	Metrics *component.ID `mapstructure:"metrics"`
	Traces  *component.ID `mapstructure:"traces"`
}

type Authentication struct {
	// Type is the authentication type. supported values are connection_string, service_principal, system_managed_identity, user_managed_identity, workload_identity, and default_credentials
	Type AuthType `mapstructure:"type"`

	// TenantID is the tenant id for the AAD App. It's only needed when type is service_principal or workload_identity.
	TenantID string `mapstructure:"tenant_id"`

	// ClientID is the AAD Application client id. It's needed when type is service_principal, user_managed_identity or workload_identity
	ClientID string `mapstructure:"client_id"`

	// ClientSecret only needed when auth type is service_principal
	ClientSecret string `mapstructure:"client_secret"`

	// ConnectionString to the Event Hubs namespace or Event Hub
	ConnectionString string `mapstructure:"connection_string"`

	// FederatedTokenFile is the path to the file containing the federated token. It's needed when type is workload_identity.
	FederatedTokenFile string `mapstructure:"federated_token_file"`
}

type AuthType string

const (
	ConnectionString      AuthType = "connection_string"
	SystemManagedIdentity AuthType = "system_managed_identity"
	UserManagedIdentity   AuthType = "user_managed_identity"
	ServicePrincipal      AuthType = "service_principal"
	WorkloadIdentity      AuthType = "workload_identity"
	DefaultCredentials    AuthType = "default_credentials"
)

type PartitionKeyConfig struct {
	// Source determines how the partition key is generated
	// Options: "static", "resource_attribute", "trace_id", "span_id", "random"
	Source string `mapstructure:"source"`

	// Value is used when source is "static" or specifies the attribute name when source is "resource_attribute"
	Value string `mapstructure:"value"`
}

// Config contains the main configuration options for the Azure Event Hubs exporter
type Config struct {
	// Namespace is the Event Hubs namespace endpoint (e.g., "my-namespace.servicebus.windows.net")
	Namespace string `mapstructure:"namespace"`

	// EventHub contains the Event Hub names for different telemetry types
	EventHub TelemetryConfig `mapstructure:"event_hub"`

	// Auth contains authentication configuration
	Auth Authentication `mapstructure:"auth"`

	// FormatType is the format of encoded telemetry data. Supported values are json and proto.
	FormatType string `mapstructure:"format"`

	// PartitionKey configuration for Event Hub partitioning
	PartitionKey PartitionKeyConfig `mapstructure:"partition_key"`

	// Encoding extension to apply for logs/metrics/traces. If present, overrides the marshaler configuration option and format.
	Encodings Encodings `mapstructure:"encodings"`

	// MaxEventSize is the maximum size of an event in bytes (default: 1MB, max: 1MB for Event Hubs)
	MaxEventSize int `mapstructure:"max_event_size"`

	// BatchSize is the number of events to batch before sending (default: 100)
	BatchSize int `mapstructure:"batch_size"`

	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

func (c *Config) Validate() error {
	if c.Namespace == "" && c.Auth.Type != ConnectionString {
		return errors.New("namespace cannot be empty when auth type is not connection_string")
	}

	switch c.Auth.Type {
	case ConnectionString:
		if c.Auth.ConnectionString == "" {
			return errors.New("connection_string cannot be empty when auth type is connection_string")
		}
	case ServicePrincipal:
		if c.Auth.TenantID == "" || c.Auth.ClientID == "" || c.Auth.ClientSecret == "" {
			return errors.New("tenant_id, client_id and client_secret cannot be empty when auth type is service_principal")
		}
	case UserManagedIdentity:
		if c.Auth.ClientID == "" {
			return errors.New("client_id cannot be empty when auth type is user_managed_identity")
		}
	case WorkloadIdentity:
		if c.Auth.TenantID == "" || c.Auth.ClientID == "" || c.Auth.FederatedTokenFile == "" {
			return errors.New("tenant_id, client_id and federated_token_file cannot be empty when auth type is workload_identity")
		}
	case DefaultCredentials:
		// No additional fields required for default credentials
		// DefaultAzureCredential will automatically detect credentials from environment
	}

	if c.FormatType != "json" && c.FormatType != "proto" {
		return errors.New("unknown format type: " + c.FormatType)
	}

	if c.MaxEventSize <= 0 || c.MaxEventSize > 1024*1024 {
		return errors.New("max_event_size must be between 1 and 1048576 bytes (1MB)")
	}

	if c.BatchSize <= 0 {
		return errors.New("batch_size must be greater than 0")
	}

	// Validate partition key configuration
	switch c.PartitionKey.Source {
	case "static":
		if c.PartitionKey.Value == "" {
			return errors.New("partition_key.value cannot be empty when source is static")
		}
	case "resource_attribute":
		if c.PartitionKey.Value == "" {
			return errors.New("partition_key.value must specify the attribute name when source is resource_attribute")
		}
	case "trace_id", "span_id", "random":
		// These don't require additional configuration
	default:
		if c.PartitionKey.Source != "" {
			return errors.New("unknown partition_key.source: " + c.PartitionKey.Source)
		}
	}

	return nil
}
