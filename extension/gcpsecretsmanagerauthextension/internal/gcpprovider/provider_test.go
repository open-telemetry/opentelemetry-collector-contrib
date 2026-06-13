// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcpprovider

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockSecretsClient struct {
	mu      sync.Mutex
	err     error
	payload []byte
	closed  bool
}

func (m *mockSecretsClient) AccessSecretVersion(_ context.Context, _ *secretmanagerpb.AccessSecretVersionRequest,
	_ ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return &secretmanagerpb.AccessSecretVersionResponse{
		Payload: &secretmanagerpb.SecretPayload{Data: m.payload},
	}, nil
}

func (m *mockSecretsClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockSecretsClient) setPayload(p []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.payload = p
}

func (m *mockSecretsClient) setErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

func TestProvider_Start_Success(t *testing.T) {
	var called bool
	mock := &mockSecretsClient{payload: []byte(`{"user":"alice","pass":"secret"}`)}
	p := NewProvider(&Config{
		Project:         "test-project",
		SecretName:      "test-secret",
		RefreshInterval: time.Minute,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, data []byte) error {
			called = true
			assert.Equal(t, `{"user":"alice","pass":"secret"}`, string(data))
			return nil
		},
	})
	p.Client = mock

	require.NoError(t, p.Start(context.Background(), nil))
	defer func() { require.NoError(t, p.Shutdown(context.Background())) }()

	assert.True(t, called)
}

func TestProvider_Start_Fails(t *testing.T) {
	mock := &mockSecretsClient{err: fmt.Errorf("access denied")}
	p := NewProvider(&Config{
		Project:         "test-project",
		SecretName:      "test-secret",
		RefreshInterval: time.Minute,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, _ []byte) error {
			t.Fatal("FetchFunc should not be called on error")
			return nil
		},
	})
	p.Client = mock

	err := p.Start(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestProvider_CredentialRotation(t *testing.T) {
	var mu sync.Mutex
	var lastValue string

	mock := &mockSecretsClient{payload: []byte("initial")}
	p := NewProvider(&Config{
		Project:         "test-project",
		SecretName:      "test-secret",
		RefreshInterval: time.Millisecond,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, data []byte) error {
			mu.Lock()
			defer mu.Unlock()
			lastValue = string(data)
			return nil
		},
	})
	p.Client = mock

	require.NoError(t, p.Start(context.Background(), nil))
	defer func() { require.NoError(t, p.Shutdown(context.Background())) }()

	mock.setPayload([]byte("rotated"))

	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return lastValue == "rotated"
	}, time.Second, 5*time.Millisecond)
}

func TestProvider_ErrorKeepsStaleState(t *testing.T) {
	var mu sync.Mutex
	var lastValue string

	mock := &mockSecretsClient{payload: []byte("good")}
	p := NewProvider(&Config{
		Project:         "test-project",
		SecretName:      "test-secret",
		RefreshInterval: time.Millisecond,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, data []byte) error {
			mu.Lock()
			defer mu.Unlock()
			lastValue = string(data)
			return nil
		},
	})
	p.Client = mock

	require.NoError(t, p.Start(context.Background(), nil))
	defer func() { require.NoError(t, p.Shutdown(context.Background())) }()

	mu.Lock()
	require.Equal(t, "good", lastValue)
	mu.Unlock()

	mock.setErr(fmt.Errorf("transient error"))
	mock.setPayload([]byte("should-not-reach"))
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, "good", lastValue)
	mu.Unlock()
}

func TestProvider_Shutdown(t *testing.T) {
	mock := &mockSecretsClient{payload: []byte("data")}
	p := NewProvider(&Config{
		Project:         "test-project",
		SecretName:      "test-secret",
		RefreshInterval: time.Minute,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, _ []byte) error {
			return nil
		},
	})
	p.Client = mock

	require.NoError(t, p.Start(context.Background(), nil))
	require.NoError(t, p.Shutdown(context.Background()))

	mock.mu.Lock()
	assert.True(t, mock.closed)
	mock.mu.Unlock()
}

func TestProvider_NilPayload(t *testing.T) {
	p := NewProvider(&Config{
		Project:         "test-project",
		SecretName:      "test-secret",
		RefreshInterval: time.Minute,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, _ []byte) error {
			t.Fatal("FetchFunc should not be called on nil payload")
			return nil
		},
	})
	p.Client = &nilPayloadClient{}

	err := p.Start(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no payload data")
}

type nilPayloadClient struct{}

func (n *nilPayloadClient) AccessSecretVersion(_ context.Context, _ *secretmanagerpb.AccessSecretVersionRequest,
	_ ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	return &secretmanagerpb.AccessSecretVersionResponse{Payload: nil}, nil
}

func (n *nilPayloadClient) Close() error { return nil }
