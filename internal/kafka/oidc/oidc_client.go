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

	// TODO support the remaining fields of clientcredentials.Config
	EndpointParams url.Values
	AuthStyle      oauth2.AuthStyle
}

func NewOIDCfileTokenProvider(ctx context.Context, clientID, clientSecretFilePath, tokenURL string,
	scopes []string, refreshAhead time.Duration,
) sarama.AccessTokenProvider {
	prov := &OIDCfileTokenProvider{
		Ctx:                  ctx,
		ClientID:             clientID,
		ClientSecretFilePath: clientSecretFilePath,
		TokenURL:             tokenURL,
		Scopes:               scopes,
		refreshAhead:         refreshAhead,
	}

	if refreshAhead.Milliseconds() > 0 {
		prov.startBackgroundRefresher()
	}

	return prov
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
		log.Printf("Skipping token refresh for %s, within the quiet window of %s",
			p.ClientID, p.refreshCooldown)
		return p.cachedToken, nil
	}

	// Read the client secret every time we get a new token,
	// as it may have changed in the meantime.
	clientSecret, err := os.ReadFile(p.ClientSecretFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read client secret: %w", err)
	}

	oauthTok, err := (&clientcredentials.Config{
		ClientID:     p.ClientID,
		ClientSecret: string(clientSecret),
		TokenURL:     p.TokenURL,
		Scopes:       p.Scopes,
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
	// log.Printf("Token refreshed for %s, will expire after %s at %s", p.ClientID,
	// expiresIn.String(), p.tokenExpiry.String())

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
	log.Printf("Will refresh access token for client %s and scope %s, %s before expiry",
		p.ClientID, p.Scopes[0], p.refreshAhead.String())
	p.backgroundOnce.Do(func() {
		go func() {
			for {
				sleepDuration := 0 * time.Minute

				p.mu.RLock()
				if p.cachedToken != nil && p.cachedToken.AccessToken != "" {
					sleepDuration = time.Until(p.tokenExpiry.Add(-p.refreshAhead))
				}
				p.mu.RUnlock()

				if sleepDuration > 0 {
					time.Sleep(sleepDuration)
					continue
				}

				if _, err := p.updateToken(); err != nil {
					log.Printf("background token refresh failed: %v\n", err)
				}
			}
		}()
	})
}

func DumpOauth2Token(tok *oauth2.Token) string {
	return fmt.Sprintf("\nAccessToken: %s...%s\nTokenType: %s\nRefreshToken: %s\nExpiry: %s\nExpiresIn: %d\n",
		tok.AccessToken[0:8], tok.AccessToken[len(tok.AccessToken)-8:], tok.TokenType,
		tok.RefreshToken, tok.Expiry.Format(time.RFC3339), tok.ExpiresIn)
}
