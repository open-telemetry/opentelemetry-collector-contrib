// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package f5cloudexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter"

import (
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

const (
	sourceHeader = "X-F5-Source"
)

type f5CloudAuthRoundTripper struct {
	transport   http.RoundTripper
	tokenSource oauth2.TokenSource
	source      string
}

func (rt *f5CloudAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone request to ensure thread safety
	req2 := req.Clone(req.Context())

	// Add authorization header
	tkn, err := rt.tokenSource.Token()
	if err != nil {
		return nil, err
	}
	tkn.SetAuthHeader(req2)

	// Add F5 specific headers
	req2.Header.Add(sourceHeader, rt.source)

	resp, err := rt.transport.RoundTrip(req2)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func newF5CloudAuthRoundTripper(ts oauth2.TokenSource, source string, next http.RoundTripper) (http.RoundTripper, error) {
	if ts == nil {
		return nil, fmt.Errorf("no TokenSource exists")
	}

	if len(source) == 0 {
		return nil, fmt.Errorf("no source provided")
	}

	rt := f5CloudAuthRoundTripper{
		transport:   next,
		tokenSource: ts,
		source:      source,
	}

	return &rt, nil
}
