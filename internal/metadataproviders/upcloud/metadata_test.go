// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upcloud

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

	p := &upcloudProviderImpl{
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

	p := &upcloudProviderImpl{
		endpoint: ts.URL,
		client:   &http.Client{},
	}

	_, err := p.Metadata(t.Context())
	assert.Error(t, err)
}

func TestQueryEndpointCorrect(t *testing.T) {
	sent := &Metadata{
		CloudName:  "upcloud",
		Hostname:   "ubuntu-1cpu-1gb-es-mad1",
		InstanceID: "00133099-f1fd-4ed2-b1c7-d027eb43a8f5",
		Region:     "es-mad1",
	}
	b, err := json.Marshal(sent)
	require.NoError(t, err)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, writeErr := w.Write(b)
		assert.NoError(t, writeErr)
	}))
	t.Cleanup(ts.Close)

	p := &upcloudProviderImpl{
		endpoint: ts.URL,
		client:   &http.Client{},
	}

	got, err := p.Metadata(t.Context())
	require.NoError(t, err)
	assert.Equal(t, *sent, *got)
}
