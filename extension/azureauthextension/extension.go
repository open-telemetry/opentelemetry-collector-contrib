// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/azureauthextension"

import (
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
)

type authenticator struct {
	credential azcore.TokenCredential
	logger     *zap.Logger
}

var (
	_ extension.Extension      = (*authenticator)(nil)
	_ extensionauth.HTTPClient = (*authenticator)(nil)
	_ extensionauth.Server     = (*authenticator)(nil)
	_ azcore.TokenCredential   = (*authenticator)(nil)
)

func newAzureAuthenticator(cfg *Config, logger *zap.Logger) (*authenticator, error) {
	var credential azcore.TokenCredential
	var err error
	failMsg := "failed to create authenticator using"

	if cfg.UseDefault {
		if credential, err = azidentity.NewDefaultAzureCredential(nil); err != nil {
			return nil, fmt.Errorf("%s default identity: %w", failMsg, err)
		}
	}

	if cfg.Workload != nil {
		if credential, err = azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			ClientID:      cfg.Workload.ClientID,
			TenantID:      cfg.Workload.TenantID,
			TokenFilePath: cfg.Workload.FederatedTokenFile,
		}); err != nil {
			return nil, fmt.Errorf("%s workload identity: %w", failMsg, err)
		}
	}

	if cfg.Managed != nil {
		clientID := cfg.Managed.ClientID
		var options *azidentity.ManagedIdentityCredentialOptions
		if clientID != "" {
			options = &azidentity.ManagedIdentityCredentialOptions{
				ID: azidentity.ClientID(clientID),
			}
		}
		if credential, err = azidentity.NewManagedIdentityCredential(options); err != nil {
			return nil, fmt.Errorf("%s managed identity: %w", failMsg, err)
		}
	}

	if cfg.ServicePrincipal != nil {
		if cfg.ServicePrincipal.ClientCertificatePath != "" {
			cert, privateKey, errParse := getCertificateAndKey(cfg.ServicePrincipal.ClientCertificatePath)
			if errParse != nil {
				return nil, fmt.Errorf("%s service principal with certificate: %w", failMsg, errParse)
			}

			if credential, err = azidentity.NewClientCertificateCredential(
				cfg.ServicePrincipal.TenantID,
				cfg.ServicePrincipal.ClientID,
				[]*x509.Certificate{cert},
				privateKey,
				nil,
			); err != nil {
				return nil, fmt.Errorf("%s service principal with certificate: %w", failMsg, err)
			}
		}
		if cfg.ServicePrincipal.ClientSecret != "" {
			if credential, err = azidentity.NewClientSecretCredential(
				cfg.ServicePrincipal.TenantID,
				cfg.ServicePrincipal.ClientID,
				cfg.ServicePrincipal.ClientSecret,
				nil,
			); err != nil {
				return nil, fmt.Errorf("%s service principal with secret: %w", failMsg, err)
			}
		}
	}

	return &authenticator{
		credential: credential,
		logger:     logger,
	}, nil
}

// getCertificateAndKey from the file
func getCertificateAndKey(filename string) (*x509.Certificate, crypto.PrivateKey, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read the certificate file: %w", err)
	}

	certs, privateKey, err := azidentity.ParseCertificates(data, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificates: %w", err)
	}

	return certs[0], privateKey, nil
}

func (a *authenticator) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (a *authenticator) Shutdown(_ context.Context) error {
	return nil
}

// GetToken returns an access token with a
// valid token for authorization
func (a *authenticator) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	if a.credential == nil {
		// This is not expected, since creating a new authenticator
		// instance returns error if the supported credentials fail
		// to initialize, and any unexpected ones should be prevented
		// from validating the config.
		return azcore.AccessToken{}, errors.New("unexpected: credentials were not initialized")
	}
	return a.credential.GetToken(ctx, options)
}

func (a *authenticator) Authenticate(ctx context.Context, _ map[string][]string) (context.Context, error) {
	// TODO
	return ctx, nil
}

func (a *authenticator) RoundTripper(_ http.RoundTripper) (http.RoundTripper, error) {
	// TODO
	// See request header: https://learn.microsoft.com/en-us/rest/api/azure/#request-header
	return nil, nil
}
