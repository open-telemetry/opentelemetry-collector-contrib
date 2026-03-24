// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsefareceiver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testIMDSResponse = "test-imds-token"

func newTestIMDSServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle IMDSv2 token request
		if r.Method == http.MethodPut && r.URL.Path == "/latest/api/token" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(testIMDSResponse))
			return
		}
		// Verify token on all other requests
		if r.Header.Get("X-aws-ec2-metadata-token") != testIMDSResponse {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}))
}

func TestGetENIID_Success(t *testing.T) {
	server := newTestIMDSServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/latest/meta-data/network/interfaces/macs/0a:1b:2c:3d:4e:5f/interface-id", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("eni-0123456789abcdef0\n"))
	})
	defer server.Close()

	resolver := &imdsENIResolver{
		client:  server.Client(),
		baseURL: server.URL,
	}

	eniID, err := resolver.GetENIID("0a:1b:2c:3d:4e:5f")
	require.NoError(t, err)
	assert.Equal(t, "eni-0123456789abcdef0", eniID)
}

func TestGetENIID_404(t *testing.T) {
	server := newTestIMDSServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer server.Close()

	resolver := &imdsENIResolver{
		client:  server.Client(),
		baseURL: server.URL,
	}

	eniID, err := resolver.GetENIID("0a:1b:2c:3d:4e:5f")
	require.Error(t, err)
	assert.Empty(t, eniID)
	assert.Contains(t, err.Error(), "IMDS returned status 404")
}

func TestGetENIID_ServerError(t *testing.T) {
	server := newTestIMDSServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer server.Close()

	resolver := &imdsENIResolver{
		client:  server.Client(),
		baseURL: server.URL,
	}

	eniID, err := resolver.GetENIID("0a:1b:2c:3d:4e:5f")
	require.Error(t, err)
	assert.Empty(t, eniID)
	assert.Contains(t, err.Error(), "IMDS returned status 500")
}

func TestGetENIID_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("too-late"))
	}))
	defer server.Close()

	resolver := &imdsENIResolver{
		client:  &http.Client{Timeout: 50 * time.Millisecond},
		baseURL: server.URL,
	}

	eniID, err := resolver.GetENIID("0a:1b:2c:3d:4e:5f")
	require.Error(t, err)
	assert.Empty(t, eniID)
}

func TestGetENIID_TokenFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/latest/api/token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	resolver := &imdsENIResolver{
		client:  server.Client(),
		baseURL: server.URL,
	}

	eniID, err := resolver.GetENIID("0a:1b:2c:3d:4e:5f")
	require.Error(t, err)
	assert.Empty(t, eniID)
	assert.Contains(t, err.Error(), "IMDS token request returned status 403")
}

func TestNewIMDSENIResolver(t *testing.T) {
	resolver := newIMDSENIResolver()
	require.NotNil(t, resolver)
	assert.Equal(t, "http://169.254.169.254", resolver.baseURL)
	assert.Equal(t, 2*time.Second, resolver.client.Timeout)
}
