// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension/internal/clientauth"

import (
	"errors"
	"fmt"
	"maps"
	"net/http"

	"golang.org/x/oauth2"
)

// roundTripper provides an HTTP RoundTripper which adds gcp credentials and
// headers.
func (ca *clientAuthenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	if ca.TokenSource == nil {
		return nil, errors.New("not started")
	}
	paramBase := &parameterTransport{
		base:   base,
		config: ca.config,
	}
	if ca.config.TokenHeader == proxyAuthorizationHeader {
		return &proxyAuthTransport{
			source: ca,
			base:   paramBase,
		}, nil
	}
	return &oauth2.Transport{
		Source: ca,
		Base:   paramBase,
	}, nil
}

type parameterTransport struct {
	base   http.RoundTripper
	config *Config
}

// RoundTrip adds Google Cloud system parameter headers to outgoing requests.
// Based on headers added by the google go client:
// https://github.com/googleapis/google-api-go-client/blob/113082d14d54f188d1b6c34c652e416592fc51b5/transport/http/dial.go#L122
func (t *parameterTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		return nil, errors.New("transport: no Transport specified")
	}
	newReq := *req
	newReq.Header = make(http.Header)
	maps.Copy(newReq.Header, req.Header)

	// Attach system parameters into the header
	if t.config.QuotaProject != "" {
		newReq.Header.Set("X-Goog-User-Project", t.config.QuotaProject)
	}
	if t.config.Project != "" {
		newReq.Header.Set("X-Goog-Project-ID", t.config.Project)
	}

	return t.base.RoundTrip(&newReq)
}

// proxyAuthTransport sets the token on the Proxy-Authorization header
// instead of the Authorization header. This is useful for IAP-protected
// endpoints where the Authorization header is used by the backend service.
type proxyAuthTransport struct {
	source oauth2.TokenSource
	base   http.RoundTripper
}

func (t *proxyAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.base == nil {
		return nil, errors.New("transport: no Transport specified")
	}
	token, err := t.source.Token()
	if err != nil {
		return nil, err
	}
	newReq := *req
	newReq.Header = make(http.Header)
	maps.Copy(newReq.Header, req.Header)
	newReq.Header.Set("Proxy-Authorization", fmt.Sprintf("%s %s", token.Type(), token.AccessToken))
	return t.base.RoundTrip(&newReq)
}
