// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Provider allows for collector extensions to be used to look up schemaURLs
type Provider interface {
	// Retrieve whill check the underlying provider to see if content exists
	// for the provided schemaURL, in the even that it doesn't an error is returned.
	Retrieve(ctx context.Context, schemaURL string) (string, error)
}

type httpProvider struct {
	client *http.Client
}

var _ Provider = (*httpProvider)(nil)

// NewHTTPProvider creates a new HTTP-based Provider.
func NewHTTPProvider(client *http.Client) Provider {
	if client == nil {
		client = http.DefaultClient
	}
	return &httpProvider{client: client}
}

func (hp *httpProvider) Retrieve(ctx context.Context, schemaURL string) (string, error) {
	if schemaURL == "" {
		return "", errors.New("schema URL cannot be empty")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, schemaURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := hp.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(data), nil
}
