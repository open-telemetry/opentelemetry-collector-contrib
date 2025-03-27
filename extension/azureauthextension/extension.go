// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/azureauthextension"

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
	"net/http"
	"os"
	"sync"
	"time"
)

type authenticator struct {
	scope      string
	credential azcore.TokenCredential

	token   *azcore.AccessToken
	tokenMu sync.RWMutex

	logger *zap.Logger
}

var (
	_ extension.Extension      = (*authenticator)(nil)
	_ extensionauth.HTTPClient = (*authenticator)(nil)
	_ extensionauth.Server     = (*authenticator)(nil)
)

func newAzureAuthenticator(cfg *Config, logger *zap.Logger) (*authenticator, error) {
	var credential azcore.TokenCredential
	var err error

	if cfg.UseDefault {
		if credential, err = azidentity.NewDefaultAzureCredential(nil); err != nil {
			return nil, fmt.Errorf("failed to create authenticator using default identity: %w", err)
		}
	}

	if cfg.Workload != nil {
		if credential, err = azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
			ClientID:      cfg.Workload.ClientID,
			TenantID:      cfg.Workload.TenantID,
			TokenFilePath: cfg.Workload.FederatedTokenFile,
		}); err != nil {
			return nil, fmt.Errorf("failed to create authenticator using workload identity: %w", err)
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
			return nil, fmt.Errorf("failed to create authenticator using managed identity: %w", err)
		}
	}

	if cfg.ServicePrincipal != nil {
		if cfg.ServicePrincipal.ClientCertificatePath != "" {
			// Read and parse the certificate
			certData, errRead := os.ReadFile(cfg.ServicePrincipal.ClientCertificatePath)
			if errRead != nil {
				return nil, fmt.Errorf("failed to read client certificate path: %w", errRead)
			}
			cert, privateKey, errParse := parseCertificateAndKey(certData)
			if errParse != nil {
				return nil, fmt.Errorf("failed to parse certificate: %w", errParse)
			}

			if credential, err = azidentity.NewClientCertificateCredential(
				cfg.ServicePrincipal.TenantID,
				cfg.ServicePrincipal.ClientID,
				[]*x509.Certificate{cert},
				privateKey,
				nil,
			); err != nil {
				return nil, fmt.Errorf("failed to create authenticator using service principal with certificate: %w", err)
			}
		}
		if cfg.ServicePrincipal.ClientSecret != "" {
			if credential, err = azidentity.NewClientSecretCredential(
				cfg.ServicePrincipal.TenantID,
				cfg.ServicePrincipal.ClientID,
				cfg.ServicePrincipal.ClientSecret,
				nil,
			); err != nil {
				return nil, fmt.Errorf("failed to create authenticator using service principal with secret: %w", err)
			}
		}
	}

	return &authenticator{
		scope:      cfg.Scope,
		credential: credential,
		token:      nil,
		tokenMu:    sync.RWMutex{},
		logger:     logger,
	}, nil
}

func parseCertificateAndKey(data []byte) (*x509.Certificate, crypto.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, nil, errors.New("failed to decode PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	return cert, privateKey, err
}

func (a *authenticator) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (a *authenticator) Shutdown(_ context.Context) error {
	return nil
}

// updateToken makes a request to get a new token
// if the authenticator does not have a token or
// it has expired.
func (a *authenticator) updateToken(ctx context.Context) (string, error) {
	a.tokenMu.RLock()
	if a.token != nil && a.token.ExpiresOn.After(time.Now().UTC()) {
		token := a.token.Token
		a.tokenMu.RUnlock()
		return token, nil
	}
	a.tokenMu.RUnlock()

	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	token, err := a.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{a.scope},
	})
	if err != nil {
		return "", fmt.Errorf("azure authenticator failed to get token: %w", err)
	}
	a.token = &token
	return token.Token, nil
}

// Authenticate adds an Authorization header
// with the bearer token
func (a *authenticator) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	var token string
	var err error

	if token, err = a.updateToken(ctx); err != nil {
		return ctx, err
	}
	// See request header: https://learn.microsoft.com/en-us/rest/api/azure/#request-header
	headers["Authorization"] = []string{"Bearer " + token}

	return ctx, nil
}

func (a *authenticator) RoundTripper(_ http.RoundTripper) (http.RoundTripper, error) {
	// TODO
	return nil, nil
}
