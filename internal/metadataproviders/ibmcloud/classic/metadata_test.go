// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package classic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// writeBody writes raw bytes to w, failing the test on error.
func writeBody(t *testing.T, w http.ResponseWriter, data []byte) {
	t.Helper()
	_, err := w.Write(data)
	require.NoError(t, err)
}

// newTestServer creates an httptest server that mimics the SoftLayer Resource Metadata API.
// Responses are plain text matching real API .txt output.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		switch r.URL.Path {
		case "/getId.txt":
			writeBody(t, w, []byte("156800198"))
		case "/getHostname.txt":
			writeBody(t, w, []byte("otel-collector"))
		case "/getDatacenter.txt":
			writeBody(t, w, []byte("par01"))
		case "/getAccountId.txt":
			writeBody(t, w, []byte("3186058"))
		case "/getGlobalIdentifier.txt":
			writeBody(t, w, []byte("06220b70-9072-4f83-ba16-d62f03106c1c"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestInstanceMetadata(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	provider := newProvider(server.URL)
	meta, err := provider.InstanceMetadata(t.Context())

	require.NoError(t, err)
	require.NotNil(t, meta)
	require.Equal(t, "156800198", meta.ID)
	require.Equal(t, "otel-collector", meta.Hostname)
	require.Equal(t, "par01", meta.Datacenter)
	require.Equal(t, "3186058", meta.AccountID)
	require.Equal(t, "06220b70-9072-4f83-ba16-d62f03106c1c", meta.GlobalIdentifier)
}

func TestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeBody(t, w, []byte("service unavailable"))
	}))
	defer server.Close()

	provider := newProvider(server.URL)
	meta, err := provider.InstanceMetadata(t.Context())

	require.Error(t, err)
	require.Nil(t, meta)
	require.Contains(t, err.Error(), "returned 503")
}

func TestEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/getId.txt":
			writeBody(t, w, []byte(""))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := newProvider(server.URL)
	meta, err := provider.InstanceMetadata(t.Context())

	// Empty ID is returned as empty string; next call fails with 404
	require.Error(t, err)
	require.Nil(t, meta)
}

func TestPartialFailure(t *testing.T) {
	// getId succeeds but getHostname fails; other endpoints return 404.
	// With parallel execution, errgroup returns whichever error completes first.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/getId.txt":
			writeBody(t, w, []byte("12345"))
		case "/getHostname.txt":
			w.WriteHeader(http.StatusInternalServerError)
			writeBody(t, w, []byte("internal error"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := newProvider(server.URL)
	meta, err := provider.InstanceMetadata(t.Context())

	require.Error(t, err)
	require.Nil(t, meta)
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := newProvider(server.URL)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	meta, err := provider.InstanceMetadata(ctx)

	require.Error(t, err)
	require.Nil(t, meta)
}

func TestNewProvider(t *testing.T) {
	provider := NewProvider()
	mc := provider.(*metadataClient)
	require.Equal(t, defaultEndpoint, mc.endpoint)
}

func TestWhitespaceHandling(t *testing.T) {
	// Verify that trailing whitespace/newlines are trimmed from responses.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/getId.txt":
			writeBody(t, w, []byte("100\n"))
		case "/getHostname.txt":
			writeBody(t, w, []byte("  test-host  \n"))
		case "/getDatacenter.txt":
			writeBody(t, w, []byte("dal13\r\n"))
		case "/getAccountId.txt":
			writeBody(t, w, []byte("200"))
		case "/getGlobalIdentifier.txt":
			writeBody(t, w, []byte("abcd-1234\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := newProvider(server.URL)
	meta, err := provider.InstanceMetadata(t.Context())

	require.NoError(t, err)
	require.NotNil(t, meta)
	require.Equal(t, "100", meta.ID)
	require.Equal(t, "test-host", meta.Hostname)
	require.Equal(t, "dal13", meta.Datacenter)
	require.Equal(t, "200", meta.AccountID)
	require.Equal(t, "abcd-1234", meta.GlobalIdentifier)
}
