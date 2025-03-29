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
	"strings"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.uber.org/zap"
)

const (
	scheme              = "Bearer"
	authorizationHeader = "Authorization"
)

var (
	errEmptyAuthorizationHeader      = errors.New("empty authorization header")
	errMissingAuthorizationHeader    = errors.New("missing authorization header")
	errUnexpectedAuthorizationFormat = errors.New(`unexpected authorization value format, expected to be of format "Bearer <token>"`)
	errUnexpectedToken               = errors.New("received token does not match the expected one")
	errUnavailableToken              = errors.New("azure authenticator has no access to token")
)

type authenticator struct {
	scope      string
	credential azcore.TokenCredential

	stopCh chan int
	token  atomic.Value

	logger *zap.Logger
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
		scope:      cfg.Scope,
		credential: credential,
		stopCh:     make(chan int, 1),
		token:      atomic.Value{},
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

func (a *authenticator) Start(ctx context.Context, _ component.Host) error {
	go a.trackToken(ctx)
	return nil
}

// updateToken makes a request to get a new token
// if the authenticator does not have a token or
// it has expired.
func (a *authenticator) updateToken(ctx context.Context) (time.Time, error) {
	if a.credential == nil {
		return time.Time{}, errors.New("authenticator does not have credentials configured")
	}
	token, err := a.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{a.scope},
	})
	if err != nil {
		// TODO Handle retries
		return time.Time{}, fmt.Errorf("azure authenticator failed to get token: %w", err)
	}
	a.token.Store(token)
	return token.ExpiresOn, nil
}

// trackToken runs on the background to refresh
// the token if it expires
func (a *authenticator) trackToken(ctx context.Context) {
	expiresOn, err := a.updateToken(ctx)
	if err != nil {
		// TODO Handle retries
		a.logger.Error("failed to update the token", zap.Error(err))
		return
	}

	getRefresh := func(expiresOn time.Time) time.Duration {
		// Refresh at 95% token lifetime
		return time.Until(expiresOn) * 95 / 100
	}

	t := time.NewTicker(getRefresh(expiresOn))
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info(
				"azure authenticator no longer refreshing the token",
				zap.String("reason", "context done"),
			)
			return
		case <-a.stopCh:
			a.logger.Info(
				"azure authenticator no longer refreshing the token",
				zap.String("reason", "received stop signal"),
			)
			close(a.stopCh)
			return
		case <-t.C:
			expiresOn, err = a.updateToken(ctx)
			if err != nil {
				// TODO Handle retries
				a.logger.Error("failed to update the token", zap.Error(err))
				a.stopCh <- 1
			} else {
				t.Reset(getRefresh(expiresOn))
			}
		}
	}
}

func (a *authenticator) Shutdown(_ context.Context) error {
	select {
	case a.stopCh <- 1:
	default: // already stopped
	}
	return nil
}

func (a *authenticator) getCurrentToken() (azcore.AccessToken, error) {
	token := a.token.Load()
	if token == nil {
		return azcore.AccessToken{}, errUnavailableToken
	}

	return token.(azcore.AccessToken), nil
}

// GetToken returns an access token with a
// valid token for authorization
func (a *authenticator) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return a.getCurrentToken()
}

// Authenticate adds an Authorization header
// with the bearer token
func (a *authenticator) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	auth, ok := headers[authorizationHeader]
	if !ok {
		auth, ok = headers[strings.ToLower(authorizationHeader)]
	}
	if !ok {
		return ctx, errMissingAuthorizationHeader
	}
	if len(auth) == 0 {
		return ctx, errEmptyAuthorizationHeader
	}

	firstAuth := strings.Split(auth[0], " ")
	if len(firstAuth) != 2 {
		return ctx, errUnexpectedAuthorizationFormat
	}
	if firstAuth[0] != scheme {
		return ctx, fmt.Errorf("expected %q scheme, got %q", scheme, firstAuth[0])
	}

	currentToken, err := a.getCurrentToken()
	if err != nil {
		return ctx, err
	}

	if firstAuth[1] != currentToken.Token {
		return ctx, errUnexpectedToken
	}

	return ctx, nil
}

func (a *authenticator) RoundTripper(_ http.RoundTripper) (http.RoundTripper, error) {
	// TODO
	// See request header: https://learn.microsoft.com/en-us/rest/api/azure/#request-header
	return nil, nil
}
