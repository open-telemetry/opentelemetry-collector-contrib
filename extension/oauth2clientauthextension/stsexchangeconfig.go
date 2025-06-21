// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"

import (
	"encoding/json"
	"fmt"
	"io"
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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &jsonResponse); err != nil {
		return nil, fmt.Errorf("failed to parse STS Server json response: %v", err)
	}

	accessToken, tokenFound := jsonResponse["access_token"]
	if !tokenFound {
		return nil, fmt.Errorf("access_token not found")
	}

	expiresIn, expiryFound := jsonResponse["expires_in"]
	if !expiryFound {
		return nil, fmt.Errorf("expires_in not found in JSON response")
	}
	expiresInFloat64, ok := expiresIn.(float64)
	if !ok {
		return nil, fmt.Errorf("expires_in type assertion failed: %v", expiresIn)
	}

	expirationTime := time.Now().Add(time.Second*time.Duration(int64(expiresInFloat64)) - jitterTime)

	stsToken, ok := accessToken.(string)
	if !ok {
		return nil, fmt.Errorf("access_token is not a string")
	}
	return &oauth2.Token{
		AccessToken: stsToken,
		Expiry:      expirationTime,
	}, nil
}
