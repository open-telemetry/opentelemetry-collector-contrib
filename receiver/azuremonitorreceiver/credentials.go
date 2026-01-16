// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

func loadTokenProvider(host component.Host, idAuth component.ID) (azcore.TokenCredential, error) {
	authExtension, ok := host.GetExtensions()[idAuth]
	if !ok {
		return nil, fmt.Errorf("unknown azureauth extension %q", idAuth.String())
	}
	credential, ok := authExtension.(azcore.TokenCredential)
	if !ok {
		return nil, fmt.Errorf("extension %q does not implement azcore.TokenCredential", idAuth.String())
	}
	return credential, nil
}

func loadCredentials(logger *zap.Logger, cfg *Config, host component.Host) (azcore.TokenCredential, error) {
	var cred azcore.TokenCredential
	var err error

	if cfg.Authentication != nil {
		logger.Info("'auth.authenticator' will be used to get the token credential")
		if cred, err = loadTokenProvider(host, cfg.Authentication.AuthenticatorID); err != nil {
			return nil, err
		}
		return cred, nil
	}

	logger.Warn("'credentials' is deprecated, use 'auth.authenticator' instead")
	switch cfg.Credentials {
	case defaultCredentials:
		if cred, err = azidentity.NewDefaultAzureCredential(nil); err != nil {
			return nil, err
		}
	case servicePrincipal:
		if cred, err = azidentity.NewClientSecretCredential(cfg.TenantID, cfg.ClientID, cfg.ClientSecret, nil); err != nil {
			return nil, err
		}
	case workloadIdentity:
		options := &azidentity.WorkloadIdentityCredentialOptions{
			ClientID:      cfg.ClientID,
			TokenFilePath: cfg.FederatedTokenFile,
			TenantID:      cfg.TenantID,
		}
		if cred, err = azidentity.NewWorkloadIdentityCredential(options); err != nil {
			return nil, err
		}
	case managedIdentity:
		var options *azidentity.ManagedIdentityCredentialOptions
		if cfg.ClientID != "" {
			options = &azidentity.ManagedIdentityCredentialOptions{
				ID: azidentity.ClientID(cfg.ClientID),
			}
		}
		if cred, err = azidentity.NewManagedIdentityCredential(options); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown credentials %v", cfg.Credentials)
	}
	return cred, nil
}
