// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// baoResponse builds the JSON body the openbao client expects for a KV read.
func baoResponse(data map[string]interface{}) []byte {
	b, _ := json.Marshal(map[string]interface{}{"data": data})
	return b
}

// newBaoTestServer returns an httptest.Server that serves a fixed response for
// every request. statusCode 200 with body is a happy path; other codes test
// error paths.
func newBaoTestServer(t *testing.T, statusCode int, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if body != nil {
			_, _ = w.Write(body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// cfg builds a BaoKeyConfig pointing at the given server address.
func baoTestCfg(addr string) *BaoKeyConfig {
	return &BaoKeyConfig{
		Address:    addr,
		Token:      "test-token",
		SecretPath: "secret/data/signing",
		CertField:  "certificate",
		KeyField:   "private_key",
	}
}

// ---------------------------------------------------------------------------
// secretField
// ---------------------------------------------------------------------------

func TestSecretFieldHappyPath(t *testing.T) {
	v, err := secretField(map[string]interface{}{"cert": "value"}, "cert")
	if err != nil || v != "value" {
		t.Errorf("expected 'value', got %q, err=%v", v, err)
	}
}

func TestSecretFieldMissing(t *testing.T) {
	_, err := secretField(map[string]interface{}{}, "cert")
	if err == nil {
		t.Error("expected error for missing field")
	}
}

func TestSecretFieldNotString(t *testing.T) {
	_, err := secretField(map[string]interface{}{"cert": 42}, "cert")
	if err == nil {
		t.Error("expected error for non-string field")
	}
}

func TestSecretFieldEmpty(t *testing.T) {
	_, err := secretField(map[string]interface{}{"cert": ""}, "cert")
	if err == nil {
		t.Error("expected error for empty field value")
	}
}

// ---------------------------------------------------------------------------
// newBaoKeyMaterialProviderWithAddress — happy path
// ---------------------------------------------------------------------------

func TestBaoProviderHappyPath(t *testing.T) {
	certPEM, keyPEM, _, _ := generateTestPEM(t)

	srv := newBaoTestServer(t, http.StatusOK, baoResponse(map[string]interface{}{
		"certificate": string(certPEM),
		"private_key": string(keyPEM),
	}))

	prov, err := newBaoKeyMaterialProviderWithAddress(context.Background(), baoTestCfg(srv.URL), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prov.GetPrivateKey() == nil {
		t.Error("private key is nil")
	}
	if prov.GetCertificate() == nil {
		t.Error("certificate is nil")
	}
}

// ---------------------------------------------------------------------------
// newBaoKeyMaterialProviderWithAddress — error paths
// ---------------------------------------------------------------------------

func TestBaoProviderConnectionError(t *testing.T) {
	// Point at a port where nothing is listening
	_, err := newBaoKeyMaterialProviderWithAddress(
		context.Background(),
		baoTestCfg("http://127.0.0.1:19998"),
		"http://127.0.0.1:19998",
	)
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestBaoProviderEmptySecret(t *testing.T) {
	// Server returns 200 with null body — openbao client returns nil secret
	srv := newBaoTestServer(t, http.StatusOK, []byte(`null`))

	_, err := newBaoKeyMaterialProviderWithAddress(context.Background(), baoTestCfg(srv.URL), srv.URL)
	if err == nil {
		t.Error("expected error for null/empty secret")
	}
}

func TestBaoProviderMissingCertField(t *testing.T) {
	_, keyPEM, _, _ := generateTestPEM(t)

	srv := newBaoTestServer(t, http.StatusOK, baoResponse(map[string]interface{}{
		// "certificate" intentionally absent
		"private_key": string(keyPEM),
	}))

	_, err := newBaoKeyMaterialProviderWithAddress(context.Background(), baoTestCfg(srv.URL), srv.URL)
	if err == nil {
		t.Error("expected error for missing certificate field")
	}
}

func TestBaoProviderMissingKeyField(t *testing.T) {
	certPEM, _, _, _ := generateTestPEM(t)

	srv := newBaoTestServer(t, http.StatusOK, baoResponse(map[string]interface{}{
		"certificate": string(certPEM),
		// "private_key" intentionally absent
	}))

	_, err := newBaoKeyMaterialProviderWithAddress(context.Background(), baoTestCfg(srv.URL), srv.URL)
	if err == nil {
		t.Error("expected error for missing private_key field")
	}
}

func TestBaoProviderBadPEM(t *testing.T) {
	srv := newBaoTestServer(t, http.StatusOK, baoResponse(map[string]interface{}{
		"certificate": "not-a-pem",
		"private_key": "not-a-pem",
	}))

	_, err := newBaoKeyMaterialProviderWithAddress(context.Background(), baoTestCfg(srv.URL), srv.URL)
	if err == nil {
		t.Error("expected error for invalid PEM data")
	}
}

func TestBaoProviderHTTPError(t *testing.T) {
	srv := newBaoTestServer(t, http.StatusForbidden, []byte(`{"errors":["permission denied"]}`))

	_, err := newBaoKeyMaterialProviderWithAddress(context.Background(), baoTestCfg(srv.URL), srv.URL)
	if err == nil {
		t.Error("expected error for HTTP 403 response")
	}
}

func TestBaoProviderNonStringField(t *testing.T) {
	certPEM, keyPEM, _, _ := generateTestPEM(t)

	srv := newBaoTestServer(t, http.StatusOK, baoResponse(map[string]interface{}{
		"certificate": string(certPEM),
		"private_key": string(keyPEM), // cert is fine but…
	}))
	// Re-use baoTestCfg but swap the field names so cert points at the integer
	cfg := baoTestCfg(srv.URL)
	cfg.CertField = "private_key"
	cfg.KeyField = "certificate"
	// both are strings so this should succeed — test that non-string is rejected
	srvBad := newBaoTestServer(t, http.StatusOK, baoResponse(map[string]interface{}{
		"certificate": 12345, // not a string
		"private_key": string(keyPEM),
	}))
	_, err := newBaoKeyMaterialProviderWithAddress(context.Background(), baoTestCfg(srvBad.URL), srvBad.URL)
	if err == nil {
		t.Error("expected error when certificate field is not a string")
	}
}
