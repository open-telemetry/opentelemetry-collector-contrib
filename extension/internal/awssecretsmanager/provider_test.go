// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanager

import (
	"context"
	"encoding/json"
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

func TestResolver_RawStringSecret(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}
	mock.setSecret("htpasswd-content-here")

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "", 5*time.Minute, zaptest.NewLogger(t), nil)
	resolver.Client = mock

	require.NoError(t, resolver.Start(t.Context()))
	defer func() { require.NoError(t, resolver.Shutdown()) }()

	assert.Equal(t, "htpasswd-content-here", resolver.Value())
}

func TestResolver_JSONKeyExtraction(t *testing.T) {
	t.Parallel()
	secret := map[string]string{"username": "admin", "password": "s3cret"}
	data, _ := json.Marshal(secret)

	mock := &mockSMClient{}
	mock.setSecret(string(data))

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "password", 5*time.Minute, zaptest.NewLogger(t), nil)
	resolver.Client = mock

	require.NoError(t, resolver.Start(t.Context()))
	defer func() { require.NoError(t, resolver.Shutdown()) }()

	assert.Equal(t, "s3cret", resolver.Value())
}

func TestResolver_MissingJSONKey(t *testing.T) {
	t.Parallel()
	secret := map[string]string{"user": "admin"}
	data, _ := json.Marshal(secret)

	mock := &mockSMClient{}
	mock.setSecret(string(data))

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "nonexistent", 5*time.Minute, zaptest.NewLogger(t), nil)
	resolver.Client = mock

	err := resolver.Start(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in secret JSON")
}

func TestResolver_FetchError(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{err: fmt.Errorf("access denied")}

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "", 5*time.Minute, zaptest.NewLogger(t), nil)
	resolver.Client = mock

	err := resolver.Start(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestResolver_NilSecretString(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "", 5*time.Minute, zaptest.NewLogger(t), nil)
	resolver.Client = mock

	err := resolver.Start(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no string value")
}

func TestResolver_Refresh(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}
	mock.setSecret("initial")

	var changed atomic.Value
	onChange := func(val string) {
		changed.Store(val)
	}

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "", 100*time.Millisecond, zaptest.NewLogger(t), onChange)
	resolver.Client = mock

	require.NoError(t, resolver.Start(t.Context()))
	defer func() { require.NoError(t, resolver.Shutdown()) }()

	assert.Equal(t, "initial", resolver.Value())

	mock.setSecret("rotated")

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "rotated", resolver.Value())
	}, 2*time.Second, 50*time.Millisecond)

	assert.Equal(t, "rotated", changed.Load())
}

func TestResolver_RefreshErrorKeepsOldValue(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}
	mock.setSecret("good-value")

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "", 100*time.Millisecond, zaptest.NewLogger(t), nil)
	resolver.Client = mock

	require.NoError(t, resolver.Start(t.Context()))
	defer func() { require.NoError(t, resolver.Shutdown()) }()

	assert.Equal(t, "good-value", resolver.Value())

	mock.err = fmt.Errorf("transient error")

	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, "good-value", resolver.Value())
}

func TestResolver_DefaultRefreshInterval(t *testing.T) {
	t.Parallel()
	resolver := NewResolver("arn", "us-east-1", "", 0, zaptest.NewLogger(t), nil)
	assert.Equal(t, DefaultRefreshInterval, resolver.refreshInterval)
}

func TestResolver_ValueBeforeStart(t *testing.T) {
	t.Parallel()
	resolver := NewResolver("arn", "us-east-1", "", 5*time.Minute, zaptest.NewLogger(t), nil)
	assert.Equal(t, "", resolver.Value())
}

func TestResolver_NonStringJSONValue(t *testing.T) {
	t.Parallel()
	data := `{"count": 42}`

	mock := &mockSMClient{}
	mock.setSecret(data)

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "count", 5*time.Minute, zaptest.NewLogger(t), nil)
	resolver.Client = mock

	err := resolver.Start(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a string")
}

func TestResolver_InvalidJSON(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}
	mock.setSecret("not-json")

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "key", 5*time.Minute, zaptest.NewLogger(t), nil)
	resolver.Client = mock

	err := resolver.Start(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse secret as JSON")
}

func TestResolver_ShutdownWithoutStart(t *testing.T) {
	t.Parallel()
	resolver := NewResolver("arn", "us-east-1", "", 5*time.Minute, zaptest.NewLogger(t), nil)
	assert.NoError(t, resolver.Shutdown())
}

func TestResolver_OnChangeNotCalledWhenValueUnchanged(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{}
	mock.setSecret("same")

	var callCount atomic.Int32
	onChange := func(_ string) {
		callCount.Add(1)
	}

	resolver := NewResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "", 100*time.Millisecond, zaptest.NewLogger(t), onChange)
	resolver.Client = mock

	require.NoError(t, resolver.Start(t.Context()))
	defer func() { require.NoError(t, resolver.Shutdown()) }()

	time.Sleep(350 * time.Millisecond)
	assert.Equal(t, int32(0), callCount.Load())
}
