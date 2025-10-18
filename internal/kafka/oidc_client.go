// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"

import (
	"context"

	"github.com/IBM/sarama"
	"golang.org/x/oauth2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"
)

// OIDCTokenProvider wraps oauth2clientauthextension to provide tokens for Kafka SASL/OAUTHBEARER
type OIDCTokenProvider struct {
	tokenSource oauth2.TokenSource
}

// Token returns a sarama.AccessToken for Sarama-based Kafka clients
func (p *OIDCTokenProvider) Token() (*sarama.AccessToken, error) {
	token, err := p.tokenSource.Token()
	if err != nil {
		return nil, err
	}
	return &sarama.AccessToken{
		Token: token.AccessToken,
	}, nil
}

// GetToken returns the oauth2.Token directly for franz-go based clients
func (p *OIDCTokenProvider) GetToken() (*oauth2.Token, error) {
	return p.tokenSource.Token()
}

// NewOIDCTokenProvider creates a new OIDC token provider using oauth2clientauthextension.
// This provides token functionality for Franz-go Kafka clients.
func NewOIDCTokenProvider(
	ctx context.Context,
	clientID string,
	clientSecretFilePath string,
	tokenURL string,
	scopes []string,
	endpointParams map[string][]string,
	authStyle oauth2.AuthStyle,
	expiryBuffer int,
) (*OIDCTokenProvider, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	config := &oauth2clientauthextension.ClientCredentialsConfig{
		Config: oauth2clientauthextension.Config{
			ClientID:         clientID,
			ClientSecretFile: clientSecretFilePath,
			TokenURL:         tokenURL,
			Scopes:           scopes,
			EndpointParams:   endpointParams,
		},
		AuthStyle:    authStyle,
		ExpiryBuffer: expiryBuffer,
	}

	tokenSource := config.TokenSource(ctx)

	return &OIDCTokenProvider{
		tokenSource: tokenSource,
	}, cancel
}
