// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
		_, err := io.Copy(w, LoadTranslationVersion(t, "complex_changeset.yml"))
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
			content, err := p.Lookup(context.Background(), tc.url)
			assert.Nil(t, content, "Expected to be nil")
			assert.Error(t, err, "Must have errored processing request")
		})
	}
}
