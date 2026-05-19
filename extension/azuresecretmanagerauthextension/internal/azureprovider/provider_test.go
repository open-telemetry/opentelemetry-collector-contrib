// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureprovider

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockSecretsClient struct {
	mu     sync.Mutex
	secret *string
	err    error
}

func (m *mockSecretsClient) GetSecret(_ context.Context, _, _ string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return azsecrets.GetSecretResponse{}, m.err
	}
	return azsecrets.GetSecretResponse{
		Secret: azsecrets.Secret{
			Value: m.secret,
		},
	}, nil
}

func (m *mockSecretsClient) setSecret(val string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secret = &val
	m.err = nil
}

func (m *mockSecretsClient) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func TestProvider_Start_Success(t *testing.T) {
	mock := &mockSecretsClient{}
	mock.setSecret(`{"user":"alice","pass":"pw"}`)

	var receivedValue string
	p := NewProvider(&Config{
		KeyVaultURI:     "https://test-vault.vault.azure.net",
		SecretName:      "test-secret",
		RefreshInterval: time.Minute,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, val string) error {
			receivedValue = val
			return nil
		},
	})
	p.Client = mock

	err := p.Start(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(t.Context()) })

	assert.Equal(t, `{"user":"alice","pass":"pw"}`, receivedValue)
}

func TestProvider_Start_Fails(t *testing.T) {
	mock := &mockSecretsClient{}
	mock.setError(errors.New("access denied"))

	p := NewProvider(&Config{
		KeyVaultURI:     "https://test-vault.vault.azure.net",
		SecretName:      "test-secret",
		RefreshInterval: time.Minute,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, _ string) error {
			t.Fatal("FetchFunc should not be called on error")
			return nil
		},
	})
	p.Client = mock

	err := p.Start(t.Context(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestProvider_CredentialRotation(t *testing.T) {
	mock := &mockSecretsClient{}
	mock.setSecret("value1")

	var lastValue atomic.Value

	p := NewProvider(&Config{
		KeyVaultURI:     "https://test-vault.vault.azure.net",
		SecretName:      "test-secret",
		RefreshInterval: time.Millisecond,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, val string) error {
			lastValue.Store(val)
			return nil
		},
	})
	p.Client = mock

	err := p.Start(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(t.Context()) })

	mock.setSecret("value2")

	assert.Eventually(t, func() bool {
		v := lastValue.Load()
		return v != nil && v.(string) == "value2"
	}, 5*time.Second, time.Millisecond)
}

func TestProvider_ErrorKeepsStaleState(t *testing.T) {
	mock := &mockSecretsClient{}
	mock.setSecret("good-value")

	var callCount atomic.Int32
	var lastValue atomic.Value

	p := NewProvider(&Config{
		KeyVaultURI:     "https://test-vault.vault.azure.net",
		SecretName:      "test-secret",
		RefreshInterval: time.Millisecond,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, val string) error {
			lastValue.Store(val)
			callCount.Add(1)
			return nil
		},
	})
	p.Client = mock

	err := p.Start(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(t.Context()) })

	initialCount := callCount.Load()
	mock.setError(errors.New("transient failure"))

	time.Sleep(50 * time.Millisecond)

	v := lastValue.Load()
	assert.Equal(t, "good-value", v.(string))
	assert.Equal(t, initialCount, callCount.Load())
}

func TestProvider_Shutdown(t *testing.T) {
	mock := &mockSecretsClient{}
	mock.setSecret("value")

	p := NewProvider(&Config{
		KeyVaultURI:     "https://test-vault.vault.azure.net",
		SecretName:      "test-secret",
		RefreshInterval: time.Millisecond,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, _ string) error {
			return nil
		},
	})
	p.Client = mock

	err := p.Start(t.Context(), nil)
	require.NoError(t, err)

	err = p.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestProvider_NilSecretValue(t *testing.T) {
	mock := &mockSecretsClient{secret: nil}

	p := NewProvider(&Config{
		KeyVaultURI:     "https://test-vault.vault.azure.net",
		SecretName:      "test-secret",
		RefreshInterval: time.Minute,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, _ string) error {
			t.Fatal("FetchFunc should not be called when secret value is nil")
			return nil
		},
	})
	p.Client = mock

	err := p.Start(t.Context(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no value")
}
