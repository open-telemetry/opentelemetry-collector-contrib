// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"context"
	"embed"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvalidHTTPProviderTests(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI != "/1.7.0" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		data := LoadTranslationVersion(t, "complex_changeset.yml")
		_, err := io.Copy(w, strings.NewReader(data))
		assert.NoError(t, err, "Must not error when trying load dataset")
	}))
	t.Cleanup(s.Close)

	tests := []struct {
		scenario string
		url      string
	}{
		{
			scenario: "A failed request happens",
			url:      fmt.Sprint(s.URL, "/not/a/valid/path/1.7.0"),
		},
		{
			scenario: "invalid url",
			url:      "unix:///localhost",
		},
	}

	for _, tc := range tests {
		t.Run(tc.scenario, func(t *testing.T) {
			p := NewHTTPProvider(s.Client())
			content, err := p.Retrieve(context.Background(), tc.url)
			assert.Empty(t, content, "Expected to be empty")
			assert.Error(t, err, "Must have errored processing request")
		})
	}
}

type testProvider struct {
	fs *embed.FS
}

func NewTestProvider(fs *embed.FS) Provider {
	return &testProvider{fs: fs}
}

func (tp testProvider) Retrieve(_ context.Context, schemaURL string) (string, error) {
	parsedPath, err := url.Parse(schemaURL)
	if err != nil {
		return "", err
	}
	f, err := tp.fs.Open(parsedPath.Path[1:])
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
