// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Provider allows for collector extensions to be used to look up schemaURLs
type Provider interface {
	// Lookup whill check the underlying provider to see if content exists
	// for the provided schemaURL, in the even that it doesn't an error is returned.
	Lookup(ctx context.Context, schemaURL string) (content io.Reader, err error)
}

type httpProvider struct {
	client *http.Client
}

var _ Provider = (*httpProvider)(nil)

func NewHTTPProvider(client *http.Client) Provider {
	return &httpProvider{client: client}
}

func (hp *httpProvider) Lookup(ctx context.Context, schemaURL string) (io.Reader, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, schemaURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := hp.client.Do(req)
	if err != nil {
		return nil, err
	}
	content := bytes.NewBuffer(nil)
	if _, err := content.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	if err := resp.Body.Close(); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid status code returned: %d", resp.StatusCode)
	}
	return content, nil
}

type testProvider struct {
	fs *embed.FS
}

func NewTestProvider(fs *embed.FS) Provider {
	return &testProvider{fs: fs}
}

func (tp testProvider) Lookup(_ context.Context, schemaURL string) (io.Reader, error) {
	parsedPath, err := url.Parse(schemaURL)
	if err != nil {
		return nil, err
	}
	f, err := tp.fs.Open(parsedPath.Path[1:])
	if err != nil {
		return nil, err
	}
	return f, nil
}
