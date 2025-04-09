// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/azureauthextension"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
)

var (
	validOptions = []string{
		"use_default",
		"managed_identity",
		"workload_identity",
		"service_principal",
	}

	errEmptyTenantID           = errors.New(`empty "tenant_id" field`)
	errEmptyClientID           = errors.New(`empty "client_id" field`)
	errEmptyClientCredential   = errors.New(`both "client_secret" and "client_certificate_path" fields are empty`)
	errEmptyFederatedTokenFile = errors.New(`empty "federated_token_file" field`)
	errEmptyAuthentication     = fmt.Errorf("authentication configuration is empty, please choose one of %s", validOptions)
	errMutuallyExclusiveAuth   = errors.New(`"client_secret" and "client_certificate_path" are mutually exclusive`)
)

type Config struct {
	Managed          *ManagedIdentity  `mapstructure:"managed_identity"`
	Workload         *WorkloadIdentity `mapstructure:"workload_identity"`
	ServicePrincipal *ServicePrincipal `mapstructure:"service_principal"`
	UseDefault       bool              `mapstructure:"use_default"`
}

type ManagedIdentity struct {
	// if left empty, then it is system managed
	ClientID string `mapstructure:"client_id"`
}

type WorkloadIdentity struct {
	ClientID           string `mapstructure:"client_id"`
	TenantID           string `mapstructure:"tenant_id"`
	FederatedTokenFile string `mapstructure:"federated_token_file"`
}

type ServicePrincipal struct {
	TenantID              string `mapstructure:"tenant_id"`
	ClientID              string `mapstructure:"client_id"`
	ClientSecret          string `mapstructure:"client_secret"`
	ClientCertificatePath string `mapstructure:"client_certificate_path"`
}

var _ component.Config = (*Config)(nil)

func (cfg *ManagedIdentity) Validate() error {
	return nil
}

func (cfg *WorkloadIdentity) Validate() error {
	var errs []error
	if cfg.TenantID == "" {
		errs = append(errs, errEmptyTenantID)
	}
	if cfg.ClientID == "" {
		errs = append(errs, errEmptyClientID)
	}
	if cfg.FederatedTokenFile == "" {
		errs = append(errs, errEmptyFederatedTokenFile)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (cfg *ServicePrincipal) Validate() error {
	var errs []error
	if cfg.TenantID == "" {
		errs = append(errs, errEmptyTenantID)
	}
	if cfg.ClientID == "" {
		errs = append(errs, errEmptyClientID)
	}
	if cfg.ClientCertificatePath == "" && cfg.ClientSecret == "" {
		errs = append(errs, errEmptyClientCredential)
	} else if cfg.ClientCertificatePath != "" && cfg.ClientSecret != "" {
		errs = append(errs, errMutuallyExclusiveAuth)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (cfg *Config) Validate() error {
	var errs []error
	if !cfg.UseDefault && cfg.ServicePrincipal == nil && cfg.Workload == nil && cfg.Managed == nil {
		errs = append(errs, errEmptyAuthentication)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
