// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidc // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/oidc"

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type OIDCfileTokenProvider struct {
	Ctx                  context.Context
	ClientID             string
	ClientSecretFilePath string
	TokenURL             string
	Scopes               []string

	mu             sync.RWMutex
	backgroundOnce sync.Once

	cachedToken *oauth2.Token

	tokenExpiry     time.Time
	lastRefreshTime time.Time

	refreshAhead    time.Duration
	refreshCooldown time.Duration

	EndpointParams url.Values
	AuthStyle      oauth2.AuthStyle
}

func (p *OIDCfileTokenProvider) Token() (*sarama.AccessToken, error) {
	oauthTok, err := p.GetToken()
	if err != nil {
		return nil, err
	}

	return &sarama.AccessToken{Token: oauthTok.AccessToken}, nil
}

func (p *OIDCfileTokenProvider) updateToken() (*oauth2.Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	if now.Sub(p.lastRefreshTime) < p.refreshCooldown {
		// Someone just refreshed - skip
		return p.cachedToken, nil
	}

	// Read the client secret every time we get a new token,
	// as it may have changed in the meantime.
	clientSecret, err := os.ReadFile(p.ClientSecretFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read client secret from %s: %w", p.ClientSecretFilePath, err)
	}

	oauthTok, err := (&clientcredentials.Config{
		ClientID:       p.ClientID,
		ClientSecret:   string(clientSecret),
		TokenURL:       p.TokenURL,
		Scopes:         p.Scopes,
		EndpointParams: p.EndpointParams,
		AuthStyle:      p.AuthStyle,
	}).Token(p.Ctx)

	if err != nil || oauthTok == nil || oauthTok.AccessToken == "" {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	// oauth2.Token() in golang.org/x/oauth2 v0.30.0 appears not to populate the `ExpiresIn` field(?) from
	// the server response.  The Expiry/`expiry` field is not standard in the OIDC or Oauth2 specs.
	var expiresIn int64
	if oauthTok.ExpiresIn != 0 {
		expiresIn = oauthTok.ExpiresIn
	} else {
		expiresIn = (oauthTok.Expiry.UnixMilli() - time.Now().UnixMilli()) / 1000
	}

	p.cachedToken = oauthTok
	p.tokenExpiry = now.Add(time.Duration(expiresIn) * time.Second)
	p.lastRefreshTime = now

	return oauthTok, nil
}

func (p *OIDCfileTokenProvider) GetToken() (*oauth2.Token, error) {
	p.mu.RLock()
	token := p.cachedToken
	expires := p.tokenExpiry
	hasToken := token != nil && token.AccessToken != ""
	p.mu.RUnlock()

	if hasToken && expires.After(time.Now()) {
		return token, nil
	}

	// No valid cached token - do a blocking refresh
	newToken, err := p.updateToken()
	if err != nil {
		return nil, err
	}

	if newToken == nil || newToken.AccessToken == "" {
		return nil, errors.New("token blank after fetch")
	}
	return newToken, nil
}

func (p *OIDCfileTokenProvider) startBackgroundRefresher() {
	p.backgroundOnce.Do(func() {
		go func() {
			for {
				p.mu.RLock()
				tokenExpiry := p.tokenExpiry

				sleepDuration := time.Until(tokenExpiry.Add(-p.refreshAhead))
				if sleepDuration <= 0 {
					sleepDuration = p.refreshCooldown
				}
				p.mu.RUnlock()

				select {
				case <-p.Ctx.Done():
					return
				case <-time.After(sleepDuration):
					if _, err := p.updateToken(); err != nil {
						log.Printf("Error: background refresher - token refresh failed: %v", err)
					}
				}
			}
		}()
	})
}

// NewOIDCTokenProvider creates a new OIDC token provider for Franz-go clients.
// This provides the same functionality as NewOIDCfileTokenProvider but returns
// a provider that can be used directly with Franz-go's oauth.Oauth mechanism.
func NewOIDCTokenProvider(ctx context.Context, clientID, clientSecretFilePath, tokenURL string,
	scopes []string, refreshAhead time.Duration, endPointParams url.Values, authStyle oauth2.AuthStyle,
) (*OIDCfileTokenProvider, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	prov := &OIDCfileTokenProvider{
		Ctx:                  ctx,
		ClientID:             clientID,
		ClientSecretFilePath: clientSecretFilePath,
		TokenURL:             tokenURL,
		Scopes:               scopes,
		refreshAhead:         refreshAhead,
		refreshCooldown:      1 * time.Second,
		EndpointParams:       endPointParams,
		AuthStyle:            authStyle,
	}

	if refreshAhead.Milliseconds() > 0 {
		prov.startBackgroundRefresher()
	}

	return prov, cancel
}
