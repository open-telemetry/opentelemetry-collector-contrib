// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanager

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockSMClient struct {
	secretString atomic.Pointer[string]
	callCount    atomic.Int32
	err          error
}

func (m *mockSMClient) GetSecretValue(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	m.callCount.Add(1)
	if m.err != nil {
		return nil, m.err
	}
	s := m.secretString.Load()
	return &secretsmanager.GetSecretValueOutput{
		SecretString: s,
	}, nil
}

func (m *mockSMClient) setSecret(s string) {
	m.secretString.Store(&s)
}

func (m *mockSMClient) clearSecret() {
	m.secretString.Store(nil)
}

func TestResolver_Start(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}
	mock.setSecret("raw-secret-content")

	var received atomic.Value
	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", 5*time.Minute, zaptest.NewLogger(t),
		func(raw string) error {
			received.Store(raw)
			return nil
		},
	)
	resolver.Client = mock

	require.NoError(t, resolver.Start(t.Context()))
	defer func() { require.NoError(t, resolver.Shutdown()) }()

	assert.Equal(t, "raw-secret-content", received.Load())
}

func TestResolver_StartFetchError(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{err: fmt.Errorf("access denied")}

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", 5*time.Minute, zaptest.NewLogger(t),
		func(_ string) error { return nil },
	)
	resolver.Client = mock

	err := resolver.Start(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestResolver_StartOnFetchError(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}
	mock.setSecret("some-value")

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", 5*time.Minute, zaptest.NewLogger(t),
		func(_ string) error { return fmt.Errorf("bad format") },
	)
	resolver.Client = mock

	err := resolver.Start(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad format")
}

func TestResolver_NilSecretString(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", 5*time.Minute, zaptest.NewLogger(t),
		func(_ string) error { return nil },
	)
	resolver.Client = mock

	err := resolver.Start(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no string value")
}

func TestResolver_Refresh(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}
	mock.setSecret("initial")

	var received atomic.Value
	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", 100*time.Millisecond, zaptest.NewLogger(t),
		func(raw string) error {
			received.Store(raw)
			return nil
		},
	)
	resolver.Client = mock

	require.NoError(t, resolver.Start(t.Context()))
	defer func() { require.NoError(t, resolver.Shutdown()) }()

	assert.Equal(t, "initial", received.Load())

	mock.setSecret("rotated")

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "rotated", received.Load())
	}, 2*time.Second, 50*time.Millisecond)
}

func TestResolver_RefreshFetchError(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}
	mock.setSecret("good-value")

	var received atomic.Value
	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", 100*time.Millisecond, zaptest.NewLogger(t),
		func(raw string) error {
			received.Store(raw)
			return nil
		},
	)
	resolver.Client = mock

	require.NoError(t, resolver.Start(t.Context()))
	defer func() { require.NoError(t, resolver.Shutdown()) }()

	assert.Equal(t, "good-value", received.Load())

	// Clear the secret so GetSecretValue returns nil SecretString, triggering a fetch error
	mock.clearSecret()
	time.Sleep(300 * time.Millisecond)

	// processSecret was never called with a new value, old value preserved by caller
	assert.Equal(t, "good-value", received.Load())
}

func TestResolver_RefreshOnFetchError(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}
	mock.setSecret("initial")

	var callCount atomic.Int32
	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", 100*time.Millisecond, zaptest.NewLogger(t),
		func(_ string) error {
			n := callCount.Add(1)
			if n > 1 {
				return fmt.Errorf("processing error")
			}
			return nil
		},
	)
	resolver.Client = mock

	require.NoError(t, resolver.Start(t.Context()))
	defer func() { require.NoError(t, resolver.Shutdown()) }()

	// Wait for at least one refresh to fire (and fail gracefully)
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.GreaterOrEqual(c, callCount.Load(), int32(2))
	}, 2*time.Second, 50*time.Millisecond)
}

func TestResolver_Shutdown(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}
	mock.setSecret("value")

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", 100*time.Millisecond, zaptest.NewLogger(t),
		func(_ string) error { return nil },
	)
	resolver.Client = mock

	require.NoError(t, resolver.Start(t.Context()))
	require.NoError(t, resolver.Shutdown())

	// After shutdown, no more fetches should happen
	countAfter := mock.callCount.Load()
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, countAfter, mock.callCount.Load())
}

func TestResolver_ShutdownWithoutStart(t *testing.T) {
	t.Parallel()
	resolver := NewResolver("arn", "us-east-1", 5*time.Minute, zaptest.NewLogger(t),
		func(_ string) error { return nil },
	)
	assert.NoError(t, resolver.Shutdown())
}

func TestResolver_DefaultRefreshInterval(t *testing.T) {
	t.Parallel()
	resolver := NewResolver("arn", "us-east-1", 0, zaptest.NewLogger(t),
		func(_ string) error { return nil },
	)
	assert.Equal(t, DefaultRefreshInterval, resolver.refreshInterval)
}
