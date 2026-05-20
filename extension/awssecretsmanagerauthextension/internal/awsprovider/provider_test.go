// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsprovider

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockSecretsManagerClient struct {
	mu     sync.Mutex
	output *secretsmanager.GetSecretValueOutput
	err    error
}

func (m *mockSecretsManagerClient) GetSecretValue(_ context.Context, _ *secretsmanager.GetSecretValueInput,
	_ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

func (m *mockSecretsManagerClient) setOutput(secret string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.output = &secretsmanager.GetSecretValueOutput{SecretString: &secret}
	m.err = nil
}

func (m *mockSecretsManagerClient) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
	m.output = nil
}

func TestProvider_Start_Success(t *testing.T) {
	secret := `{"user":"admin","pass":"secret123"}`
	mock := &mockSecretsManagerClient{}
	mock.setOutput(secret)

	var receivedValue string
	p := NewProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Minute,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, val string) error {
			receivedValue = val
			return nil
		},
	})
	p.Client = mock

	err := p.Start(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	assert.Equal(t, secret, receivedValue)
}

func TestProvider_Start_Fails(t *testing.T) {
	mock := &mockSecretsManagerClient{}
	mock.setError(errors.New("access denied"))

	p := NewProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Minute,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, _ string) error {
			t.Fatal("FetchFunc should not be called on error")
			return nil
		},
	})
	p.Client = mock

	err := p.Start(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestProvider_CredentialRotation(t *testing.T) {
	mock := &mockSecretsManagerClient{}
	mock.setOutput("value1")

	var callCount atomic.Int32
	var lastValue atomic.Value

	p := NewProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Millisecond,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, val string) error {
			lastValue.Store(val)
			callCount.Add(1)
			return nil
		},
	})
	p.Client = mock

	err := p.Start(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	mock.setOutput("value2")

	assert.Eventually(t, func() bool {
		v := lastValue.Load()
		return v != nil && v.(string) == "value2"
	}, 5*time.Second, time.Millisecond)
}

func TestProvider_ErrorKeepsStaleState(t *testing.T) {
	mock := &mockSecretsManagerClient{}
	mock.setOutput("good-value")

	var callCount atomic.Int32
	var lastValue atomic.Value

	p := NewProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Millisecond,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, val string) error {
			lastValue.Store(val)
			callCount.Add(1)
			return nil
		},
	})
	p.Client = mock

	err := p.Start(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	initialCount := callCount.Load()
	mock.setError(errors.New("transient failure"))

	// Wait for at least one refresh cycle
	time.Sleep(50 * time.Millisecond)

	// FetchFunc should NOT have been called again since the error
	// (the last successful call stored "good-value")
	v := lastValue.Load()
	assert.Equal(t, "good-value", v.(string))

	// Verify refresh was attempted (count didn't increase because error path doesn't call FetchFunc)
	assert.Equal(t, initialCount, callCount.Load())
}

func TestProvider_Shutdown(t *testing.T) {
	mock := &mockSecretsManagerClient{}
	mock.setOutput("value")

	p := NewProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Millisecond,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, _ string) error {
			return nil
		},
	})
	p.Client = mock

	err := p.Start(context.Background(), nil)
	require.NoError(t, err)

	err = p.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestProvider_NilSecretString(t *testing.T) {
	mock := &mockSecretsManagerClient{
		output: &secretsmanager.GetSecretValueOutput{SecretString: nil},
	}

	p := NewProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Minute,
		Logger:          zaptest.NewLogger(t),
		FetchFunc: func(_ context.Context, _ string) error {
			t.Fatal("FetchFunc should not be called when SecretString is nil")
			return nil
		},
	})
	p.Client = mock

	err := p.Start(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no string value")
}
