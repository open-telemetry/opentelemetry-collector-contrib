// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

type stsExchangeConfig struct {
	Config
}

const (
	jitterTime = time.Second * time.Duration(10)
)

// stsTokenSource implements the interface `oauth2.TokenSource`.
type stsTokenSource struct {
	config    *stsExchangeConfig
	transport http.RoundTripper
}

var _ oauth2.TokenSource = &stsTokenSource{}

func newStsTokenSource(config *Config, transport http.RoundTripper) (oauth2.TokenSource, error) {
	ts := &stsTokenSource{
		config: &stsExchangeConfig{
			Config: *config,
		},
		transport: transport,
	}
	return oauth2.ReuseTokenSource(nil, ts), nil
}

// Token reads subject token and exchanges for STS token
func (ts stsTokenSource) Token() (*oauth2.Token, error) {
	subjectToken, err := getActualValue(ts.config.SubjectToken, ts.config.SubjectTokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read security token from file: %v", err)
	}

	data := url.Values{
		"grant_type":           {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"audience":             {ts.config.Audience},
		"scope":                {strings.Join(ts.config.Scopes, " ")},
		"requested_token_type": {"urn:ietf:params:oauth:token-type:access_token"},
		"subject_token":        {subjectToken},
		"subject_token_type":   {ts.config.SubjectTokenType},
	}
	req, err := http.NewRequest(http.MethodPost, ts.config.TokenURL, strings.NewReader(data.Encode()))

	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Connection", "close")

	var client = http.Client{
		Transport: ts.transport,
		Timeout:   ts.config.Timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange token: %v", err)
	}
	defer resp.Body.Close()

	var token oauth2.Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode token from response: %v", err)
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("invalid token response: access_token missing")
	}
	if token.ExpiresIn != 0 {
		now := time.Now()
		token.Expiry = now.Add(time.Second*time.Duration(token.ExpiresIn) - jitterTime)
		if token.Expiry.Before(now) {
			return nil, fmt.Errorf("invalid token response: token expired")
		}
	}
	return &token, nil
}
