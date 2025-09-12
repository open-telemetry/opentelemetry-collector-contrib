// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vultr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider(t *testing.T) {
	p := NewProvider()
	assert.NotNil(t, p)
}

func TestQueryEndpointFailed(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(ts.Close)

	p := &vultrProviderImpl{
		endpoint: ts.URL,
		client:   &http.Client{},
	}

	_, err := p.Metadata(t.Context())
	assert.Error(t, err)
}

func TestQueryEndpointMalformed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprintln(w, "{") // invalid JSON
		assert.NoError(t, err)
	}))
	t.Cleanup(ts.Close)

	p := &vultrProviderImpl{
		endpoint: ts.URL,
		client:   &http.Client{},
	}

	_, err := p.Metadata(t.Context())
	assert.Error(t, err)
}

func TestQueryEndpointCorrect(t *testing.T) {
	sent := &Metadata{
		Hostname:     "vultr-guest",
		InstanceID:   "a747bfz6385e",
		InstanceV2ID: "36e9cf60-5d93-4e31-8ebf-613b3d2874fb",
		Region:       Region{RegionCode: "EWR"},
	}
	b, err := json.Marshal(sent)
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, writeErr := w.Write(b)
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(ts.Close)

	p := &vultrProviderImpl{
		endpoint: ts.URL,
		client:   &http.Client{},
	}

	got, err := p.Metadata(t.Context())
	require.NoError(t, err)
	assert.Equal(t, *sent, *got)
}
