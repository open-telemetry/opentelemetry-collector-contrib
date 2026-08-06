// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"context"
	"testing"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// makeSecret builds a corev1.Secret pre-populated with data fields.
func makeSecret(name, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       data,
	}
}

// ---------------------------------------------------------------------------
// fetchSecretDataWithClient
// ---------------------------------------------------------------------------

func TestFetchSecretDataWithClientHappyPath(t *testing.T) {
	certPEM, keyPEM, _, _ := generateTestPEM(t)
	client := fake.NewSimpleClientset(makeSecret("signing-secret", "default", map[string][]byte{
		"tls.crt": certPEM,
		"tls.key": keyPEM,
	}))

	data, err := fetchSecretDataWithClient(context.Background(), client, "signing-secret", "default", "tls.crt", zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != string(certPEM) {
		t.Error("returned data does not match expected cert PEM")
	}
}

func TestFetchSecretDataWithClientMissingKey(t *testing.T) {
	certPEM, _, _, _ := generateTestPEM(t)
	client := fake.NewSimpleClientset(makeSecret("signing-secret", "default", map[string][]byte{
		"tls.crt": certPEM,
		// "tls.key" intentionally absent
	}))

	_, err := fetchSecretDataWithClient(context.Background(), client, "signing-secret", "default", "tls.key", zap.NewNop())
	if err == nil {
		t.Error("expected error for missing key in secret")
	}
}

func TestFetchSecretDataWithClientSecretNotFound(t *testing.T) {
	// The fake client returns a real not-found error on every GET. The retry
	// loop would normally sleep and retry 30 times; we cancel the context
	// immediately so the sleep is interrupted and we get an error fast.
	client := fake.NewSimpleClientset() // no secrets at all

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled — first Get returns, then Sleep is interrupted by ctx

	_, err := fetchSecretDataWithClient(ctx, client, "signing-secret", "default", "tls.crt", zap.NewNop())
	if err == nil {
		t.Error("expected error for non-existent secret")
	}
}

func TestFetchSecretDataWithClientNilLogger(t *testing.T) {
	certPEM, keyPEM, _, _ := generateTestPEM(t)
	client := fake.NewSimpleClientset(makeSecret("s", "ns", map[string][]byte{
		"cert": certPEM,
		"key":  keyPEM,
	}))

	// nil logger must not panic
	data, err := fetchSecretDataWithClient(context.Background(), client, "s", "ns", "cert", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}
}

// ---------------------------------------------------------------------------
// newK8sKeyMaterialProviderWithClient
// ---------------------------------------------------------------------------

func TestK8sKeyMaterialProviderHappyPath(t *testing.T) {
	certPEM, keyPEM, _, _ := generateTestPEM(t)
	client := fake.NewSimpleClientset(makeSecret("signing-secret", "default", map[string][]byte{
		"tls.crt": certPEM,
		"tls.key": keyPEM,
	}))

	cfg := &K8sSecretConfig{
		Name:      "signing-secret",
		Namespace: "default",
		CertKey:   "tls.crt",
		KeyKey:    "tls.key",
	}
	prov, err := newK8sKeyMaterialProviderWithClient(context.Background(), client, cfg, zap.NewNop())
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

func TestK8sKeyMaterialProviderCertFetchError(t *testing.T) {
	// Secret exists but cert key is absent
	_, keyPEM, _, _ := generateTestPEM(t)
	client := fake.NewSimpleClientset(makeSecret("signing-secret", "default", map[string][]byte{
		"tls.key": keyPEM,
		// tls.crt missing
	}))

	cfg := &K8sSecretConfig{
		Name:      "signing-secret",
		Namespace: "default",
		CertKey:   "tls.crt",
		KeyKey:    "tls.key",
	}
	_, err := newK8sKeyMaterialProviderWithClient(context.Background(), client, cfg, zap.NewNop())
	if err == nil {
		t.Error("expected error when cert key is missing")
	}
}

func TestK8sKeyMaterialProviderKeyFetchError(t *testing.T) {
	// Secret exists but key field is absent
	certPEM, _, _, _ := generateTestPEM(t)
	client := fake.NewSimpleClientset(makeSecret("signing-secret", "default", map[string][]byte{
		"tls.crt": certPEM,
		// tls.key missing
	}))

	cfg := &K8sSecretConfig{
		Name:      "signing-secret",
		Namespace: "default",
		CertKey:   "tls.crt",
		KeyKey:    "tls.key",
	}
	_, err := newK8sKeyMaterialProviderWithClient(context.Background(), client, cfg, zap.NewNop())
	if err == nil {
		t.Error("expected error when key field is missing")
	}
}

func TestK8sKeyMaterialProviderBadPEM(t *testing.T) {
	client := fake.NewSimpleClientset(makeSecret("signing-secret", "default", map[string][]byte{
		"tls.crt": []byte("not-a-pem"),
		"tls.key": []byte("not-a-pem"),
	}))

	cfg := &K8sSecretConfig{
		Name:      "signing-secret",
		Namespace: "default",
		CertKey:   "tls.crt",
		KeyKey:    "tls.key",
	}
	_, err := newK8sKeyMaterialProviderWithClient(context.Background(), client, cfg, zap.NewNop())
	if err == nil {
		t.Error("expected error for invalid PEM data")
	}
}
