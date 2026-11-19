// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func makeSecret(name, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       data,
	}
}

func TestFetchSecretDataWithClientHappyPath(t *testing.T) {
	_, _, certPEM := newTestRSA(t)
	client := fake.NewSimpleClientset(makeSecret("verify-secret", "default", map[string][]byte{
		"tls.crt": certPEM,
	}))

	data, err := fetchSecretDataWithClient(t.Context(), client, "verify-secret", "default", "tls.crt", zap.NewNop())
	require.NoError(t, err)
	assert.True(t, bytes.Equal(data, certPEM))
}

func TestFetchSecretDataWithClientMissingKey(t *testing.T) {
	client := fake.NewSimpleClientset(makeSecret("verify-secret", "default", map[string][]byte{
		"tls.crt": []byte("cert"),
	}))

	_, err := fetchSecretDataWithClient(t.Context(), client, "verify-secret", "default", "hmac.key", zap.NewNop())
	require.Error(t, err)
}

func TestFetchSecretDataWithClientSecretNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := fetchSecretDataWithClient(ctx, client, "verify-secret", "default", "tls.crt", zap.NewNop())
	require.Error(t, err)
}

func TestFetchSecretDataWithClientNilLogger(t *testing.T) {
	client := fake.NewSimpleClientset(makeSecret("s", "ns", map[string][]byte{
		"cert": []byte("pem"),
	}))

	data, err := fetchSecretDataWithClient(t.Context(), client, "s", "ns", "cert", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestFetchSecretDataWithClientNonRetryable(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("get", "secrets", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})

	_, err := fetchSecretDataWithClient(t.Context(), client, "verify-secret", "default", "tls.crt", zap.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden")
}

func TestK8sKeyMaterialProviderCertificate(t *testing.T) {
	_, cert, certPEM := newTestRSA(t)
	client := fake.NewSimpleClientset(makeSecret("verify-secret", "default", map[string][]byte{
		"tls.crt": certPEM,
	}))

	prov, err := newK8sKeyMaterialProviderWithClient(t.Context(), client, &K8sSecretConfig{
		Name:         "verify-secret",
		Namespace:    "default",
		SecretConfig: SecretConfig{Certificate: "tls.crt"},
	}, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, prov.GetCertificate())
	assert.Equal(t, cert.Raw, prov.GetCertificate().Raw)
	assert.Empty(t, prov.GetHMACKey())
}

func TestK8sKeyMaterialProviderBase64Certificate(t *testing.T) {
	_, cert, certPEM := newTestRSA(t)
	encoded := []byte(base64.StdEncoding.EncodeToString(certPEM))
	client := fake.NewSimpleClientset(makeSecret("verify-secret", "audit", map[string][]byte{
		"tls.crt": encoded,
	}))

	prov, err := newK8sKeyMaterialProviderWithClient(t.Context(), client, &K8sSecretConfig{
		Name:         "verify-secret",
		Namespace:    "audit",
		SecretConfig: SecretConfig{Certificate: "tls.crt"},
	}, zap.NewNop())
	require.NoError(t, err)
	assert.Equal(t, cert.Raw, prov.GetCertificate().Raw)
}

func TestK8sKeyMaterialProviderMissingCertificate(t *testing.T) {
	client := fake.NewSimpleClientset(makeSecret("verify-secret", "default", map[string][]byte{
		"other": []byte("x"),
	}))

	_, err := newK8sKeyMaterialProviderWithClient(t.Context(), client, &K8sSecretConfig{
		Name:         "verify-secret",
		Namespace:    "default",
		SecretConfig: SecretConfig{Certificate: "tls.crt"},
	}, zap.NewNop())
	require.Error(t, err)
}

func TestK8sKeyMaterialProviderBadPEM(t *testing.T) {
	client := fake.NewSimpleClientset(makeSecret("verify-secret", "default", map[string][]byte{
		"tls.crt": []byte("not-a-pem"),
	}))

	_, err := newK8sKeyMaterialProviderWithClient(t.Context(), client, &K8sSecretConfig{
		Name:         "verify-secret",
		Namespace:    "default",
		SecretConfig: SecretConfig{Certificate: "tls.crt"},
	}, zap.NewNop())
	require.Error(t, err)
}

func TestK8sKeyMaterialProviderHMAC(t *testing.T) {
	rawKey := []byte("supersecrethmackey32byteslong!!!!")
	client := fake.NewSimpleClientset(makeSecret("verify-secret", "default", map[string][]byte{
		"hmac.key": rawKey,
	}))

	prov, err := newK8sKeyMaterialProviderWithClient(t.Context(), client, &K8sSecretConfig{
		Name:         "verify-secret",
		Namespace:    "default",
		SecretConfig: SecretConfig{HMACKey: "hmac.key"},
	}, zap.NewNop())
	require.NoError(t, err)
	assert.Equal(t, rawKey, prov.GetHMACKey())
	assert.Nil(t, prov.GetCertificate())
}

func TestK8sKeyMaterialProviderHMACEmpty(t *testing.T) {
	client := fake.NewSimpleClientset(makeSecret("verify-secret", "default", map[string][]byte{
		"hmac.key": {},
	}))

	_, err := newK8sKeyMaterialProviderWithClient(t.Context(), client, &K8sSecretConfig{
		Name:         "verify-secret",
		Namespace:    "default",
		SecretConfig: SecretConfig{HMACKey: "hmac.key"},
	}, zap.NewNop())
	require.Error(t, err)
}

func TestK8sProviderWithoutCluster(t *testing.T) {
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	_, err := newKeyMaterialProvider(t.Context(), &Config{
		KeySource: KeySourceConfig{
			Type: keySourceK8sSecret,
			K8sSecret: &K8sSecretConfig{
				Name:         "verify-secret",
				Namespace:    "default",
				SecretConfig: SecretConfig{HMACKey: "hmac.key"},
			},
		},
	}, zap.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create k8s client")
}
