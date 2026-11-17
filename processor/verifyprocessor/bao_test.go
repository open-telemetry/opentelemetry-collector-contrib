// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/zap"
)

func baoResponse(t *testing.T, data map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"data": map[string]any{
			"data": data,
			"metadata": map[string]any{
				"version":       1,
				"created_time":  "2024-01-01T00:00:00Z",
				"deletion_time": "",
				"destroyed":     false,
			},
		},
	})
	require.NoError(t, err)
	return b
}

func newBaoTestServer(t *testing.T, statusCode int, body []byte) *httptest.Server {
	t.Helper()
	return newBaoTestServerCheck(t, statusCode, body, nil)
}

func newBaoTestServerCheck(t *testing.T, statusCode int, body []byte, check func(*http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			check(r)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if body != nil {
			_, _ = w.Write(body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSecretField(t *testing.T) {
	v, err := secretField(map[string]any{"cert": "value"}, "cert")
	require.NoError(t, err)
	assert.Equal(t, "value", v)

	_, err = secretField(map[string]any{}, "cert")
	require.Error(t, err)

	_, err = secretField(map[string]any{"cert": 42}, "cert")
	require.Error(t, err)

	_, err = secretField(map[string]any{"cert": ""}, "cert")
	require.Error(t, err)
}

func TestBaoProviderCertificate(t *testing.T) {
	_, cert, certPEM := newTestRSA(t)
	var sawToken bool
	srv := newBaoTestServerCheck(t, http.StatusOK, baoResponse(t, map[string]any{
		"certificate": string(certPEM),
	}), func(r *http.Request) {
		sawToken = r.Header.Get("X-Vault-Token") == "test-token"
		assert.Contains(t, r.URL.Path, "/secret/data/verify")
	})

	prov, err := newBaoKeyMaterialProviderWithAddress(t.Context(), &BaoKeyConfig{
		Address:      srv.URL,
		Token:        configopaque.String("test-token"),
		MountPath:    "secret",
		SecretPath:   "verify",
		SecretConfig: SecretConfig{Certificate: "certificate"},
	}, srv.URL)
	require.NoError(t, err)
	assert.True(t, sawToken)
	require.NotNil(t, prov.GetCertificate())
	assert.Equal(t, cert.Raw, prov.GetCertificate().Raw)
	assert.Empty(t, prov.GetHMACKey())
}

func TestBaoProviderBase64Certificate(t *testing.T) {
	_, cert, certPEM := newTestRSA(t)
	srv := newBaoTestServer(t, http.StatusOK, baoResponse(t, map[string]any{
		"certificate": base64.StdEncoding.EncodeToString(certPEM),
	}))

	prov, err := newBaoKeyMaterialProvider(t.Context(), &BaoKeyConfig{
		Address:      srv.URL,
		MountPath:    "secret",
		SecretPath:   "verify",
		SecretConfig: SecretConfig{Certificate: "certificate"},
	})
	require.NoError(t, err)
	assert.Equal(t, cert.Raw, prov.GetCertificate().Raw)
}

func TestBaoProviderHMAC(t *testing.T) {
	raw := []byte("bao-hmac-secret")
	srv := newBaoTestServer(t, http.StatusOK, baoResponse(t, map[string]any{
		"hmac_key": base64.StdEncoding.EncodeToString(raw),
	}))

	prov, err := newKeyMaterialProvider(t.Context(), &Config{
		KeySource: KeySourceConfig{
			Type: keySourceBao,
			Bao: &BaoKeyConfig{
				Address:      srv.URL,
				SecretPath:   "verify",
				SecretConfig: SecretConfig{HMACKey: "hmac_key"},
			},
		},
	}, zap.NewNop())
	require.NoError(t, err)
	assert.Equal(t, raw, prov.GetHMACKey())
	assert.Nil(t, prov.GetCertificate())
}

func TestBaoProviderHMACBadEncoding(t *testing.T) {
	srv := newBaoTestServer(t, http.StatusOK, baoResponse(t, map[string]any{
		"hmac_key": "not base64!!!",
	}))

	_, err := newBaoKeyMaterialProviderWithAddress(t.Context(), &BaoKeyConfig{
		MountPath:    "secret",
		SecretPath:   "verify",
		SecretConfig: SecretConfig{HMACKey: "hmac_key"},
	}, srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "standard base64")
}

func TestBaoProviderDefaultMount(t *testing.T) {
	raw := []byte("k")
	srv := newBaoTestServerCheck(t, http.StatusOK, baoResponse(t, map[string]any{
		"hmac_key": base64.StdEncoding.EncodeToString(raw),
	}), func(r *http.Request) {
		assert.Contains(t, r.URL.Path, "/secret/data/verify")
	})

	prov, err := newBaoKeyMaterialProviderWithAddress(t.Context(), &BaoKeyConfig{
		SecretPath:   "verify",
		SecretConfig: SecretConfig{HMACKey: "hmac_key"},
	}, srv.URL)
	require.NoError(t, err)
	assert.Equal(t, raw, prov.GetHMACKey())
}

func TestBaoProviderMissingCertificate(t *testing.T) {
	srv := newBaoTestServer(t, http.StatusOK, baoResponse(t, map[string]any{
		"other": "value",
	}))

	_, err := newBaoKeyMaterialProviderWithAddress(t.Context(), &BaoKeyConfig{
		MountPath:    "secret",
		SecretPath:   "verify",
		SecretConfig: SecretConfig{Certificate: "certificate"},
	}, srv.URL)
	require.Error(t, err)
}

func TestBaoProviderBadPEM(t *testing.T) {
	srv := newBaoTestServer(t, http.StatusOK, baoResponse(t, map[string]any{
		"certificate": "not-a-pem",
	}))

	_, err := newBaoKeyMaterialProviderWithAddress(t.Context(), &BaoKeyConfig{
		MountPath:    "secret",
		SecretPath:   "verify",
		SecretConfig: SecretConfig{Certificate: "certificate"},
	}, srv.URL)
	require.Error(t, err)
}

func TestBaoProviderNotFound(t *testing.T) {
	srv := newBaoTestServer(t, http.StatusNotFound, []byte(`{"errors":[]}`))

	_, err := newBaoKeyMaterialProviderWithAddress(t.Context(), &BaoKeyConfig{
		MountPath:    "secret",
		SecretPath:   "verify",
		SecretConfig: SecretConfig{Certificate: "certificate"},
	}, srv.URL)
	require.Error(t, err)
}

func TestBaoProviderHTTPError(t *testing.T) {
	srv := newBaoTestServer(t, http.StatusForbidden, []byte(`{"errors":["permission denied"]}`))

	_, err := newBaoKeyMaterialProviderWithAddress(t.Context(), &BaoKeyConfig{
		MountPath:    "secret",
		SecretPath:   "verify",
		SecretConfig: SecretConfig{HMACKey: "hmac_key"},
	}, srv.URL)
	require.Error(t, err)
}

func TestBaoProviderNonStringField(t *testing.T) {
	srv := newBaoTestServer(t, http.StatusOK, baoResponse(t, map[string]any{
		"certificate": 12345,
	}))

	_, err := newBaoKeyMaterialProviderWithAddress(t.Context(), &BaoKeyConfig{
		MountPath:    "secret",
		SecretPath:   "verify",
		SecretConfig: SecretConfig{Certificate: "certificate"},
	}, srv.URL)
	require.Error(t, err)
}

func TestBaoProviderConnectionError(t *testing.T) {
	_, err := newBaoKeyMaterialProviderWithAddress(
		t.Context(),
		&BaoKeyConfig{
			MountPath:    "secret",
			SecretPath:   "verify",
			SecretConfig: SecretConfig{HMACKey: "hmac_key"},
		},
		"http://127.0.0.1:1",
	)
	require.Error(t, err)
}
