// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestGetProxyFunc(t *testing.T) {
	t.Run("ExplicitProxyAddressWins", func(t *testing.T) {
		t.Setenv("HTTP_PROXY", "http://env-http-proxy:8080")
		t.Setenv("HTTPS_PROXY", "http://env-https-proxy:8080")
		t.Setenv("NO_PROXY", "")

		fn, err := getProxyFunc("http://explicit:9999")
		require.NoError(t, err)
		require.NotNil(t, fn)

		req, _ := http.NewRequest("GET", "https://anything.example.com/", nil)
		u, err := fn(req)
		require.NoError(t, err)
		assert.Equal(t, "http://explicit:9999", u.String())
	})

	t.Run("EmptyFallsThroughToEnvironment", func(t *testing.T) {
		t.Setenv("HTTPS_PROXY", "http://env-https-proxy:8080")
		t.Setenv("NO_PROXY", "")

		fn, err := getProxyFunc("")
		require.NoError(t, err)
		require.NotNil(t, fn)

		req, _ := http.NewRequest("GET", "https://anything.example.com/", nil)
		u, err := fn(req)
		require.NoError(t, err)
		require.NotNil(t, u)
		assert.Equal(t, "http://env-https-proxy:8080", u.String())
	})

	t.Run("InvalidProxyAddressReturnsError", func(t *testing.T) {
		fn, err := getProxyFunc("http://bad-percent-encoding%")
		assert.Error(t, err)
		assert.Nil(t, fn)
	})
}

func TestLoadCertPool(t *testing.T) {
	t.Run("ValidPEM", func(t *testing.T) {
		pool, err := loadCertPool(filepath.Join("testdata", "public_amazon_cert.pem"))
		require.NoError(t, err)
		assert.NotNil(t, pool)
	})

	t.Run("MissingFile", func(t *testing.T) {
		_, err := loadCertPool(filepath.Join(t.TempDir(), "no_such_file.pem"))
		assert.Error(t, err)
	})

	t.Run("EmptyPath", func(t *testing.T) {
		_, err := loadCertPool("")
		assert.Error(t, err)
	})

	t.Run("MalformedPEM", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "junk.pem")
		require.NoError(t, os.WriteFile(f, []byte("this is not a PEM"), 0o600))
		_, err := loadCertPool(f)
		assert.Error(t, err)
	})
}

func TestNewHTTPClient(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		client, err := newHTTPClient(zap.NewNop(), 8, 30, false, "", "")
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("InvalidProxyPropagatesError", func(t *testing.T) {
		_, err := newHTTPClient(zap.NewNop(), 8, 30, false, "http://bad-percent%", "")
		assert.Error(t, err)
	})

	t.Run("InvalidCertFileLogsAndContinues", func(t *testing.T) {
		// A typo in CertificateFilePath logs a warning and falls back to
		// system trust rather than failing client construction.
		client, err := newHTTPClient(zap.NewNop(), 8, 30, false, "", filepath.Join(t.TempDir(), "missing"))
		require.NoError(t, err)
		require.NotNil(t, client)
	})
}

func TestProxyServerTransport(t *testing.T) {
	cfg := &AWSSessionSettings{
		NumberOfWorkers:       8,
		RequestTimeoutSeconds: 30,
		NoVerifySSL:           true,
	}
	tr, err := ProxyServerTransport(zap.NewNop(), cfg)
	require.NoError(t, err)
	require.NotNil(t, tr)
	assert.True(t, tr.DisableCompression)
	assert.Equal(t, 8, tr.MaxIdleConns)
	assert.Equal(t, 8, tr.MaxIdleConnsPerHost)
	require.NotNil(t, tr.TLSClientConfig)
	assert.True(t, tr.TLSClientConfig.InsecureSkipVerify)
}
