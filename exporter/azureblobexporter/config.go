// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"go.opentelemetry.io/collector/component"
)

type Container struct {
	Logs    string `mapstructure:"logs"`
	Metrics string `mapstructure:"metrics"`
	Traces  string `mapstructure:"traces"`
}

type Authentication struct {
	Type     AuthType `mapstructure:"type"`
	TenantId string   `mapstructure:"tenant_id"`
	ClientId string   `mapstructure:"client_id"`
	// only needed when auth type is service_principal
	ClientSecret string `mapstructure:"client_secret"`
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
	Url       string         `mapstructure:"url"`
	Container Container      `mapstructure:"container"`
	Auth      Authentication `mapstructure:"auth"`
	// Encoding to apply. If present, overrides the marshaler configuration option.
	Encoding *component.ID `mapstructure:"encoding"`
}

func (c *Config) Validate() error {
	// TODO to be implemented
	return nil
}
