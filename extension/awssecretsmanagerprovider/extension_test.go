// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerprovider

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

type mockSMClient struct {
	mu     sync.Mutex
	output *secretsmanager.GetSecretValueOutput
	err    error
}

func (m *mockSMClient) GetSecretValue(_ context.Context, _ *secretsmanager.GetSecretValueInput,
	_ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

func (m *mockSMClient) setOutput(secret string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.output = &secretsmanager.GetSecretValueOutput{SecretString: &secret}
	m.err = nil
}

func (m *mockSMClient) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
	m.output = nil
}

func TestStart_Success(t *testing.T) {
	secret := `{"user":"admin","pass":"secret123"}`
	mock := &mockSMClient{}
	mock.setOutput(secret)

	p := newAWSSecretProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Minute,
	}, zaptest.NewLogger(t))
	p.client = mock

	err := p.Start(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	val, err := p.GetSecret(context.Background())
	require.NoError(t, err)
	assert.Equal(t, secret, val)
}

func TestStart_Failure(t *testing.T) {
	mock := &mockSMClient{}
	mock.setError(errors.New("access denied"))

	p := newAWSSecretProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Minute,
	}, zaptest.NewLogger(t))
	p.client = mock

	err := p.Start(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestStart_NilSecretString(t *testing.T) {
	mock := &mockSMClient{
		output: &secretsmanager.GetSecretValueOutput{SecretString: nil},
	}

	p := newAWSSecretProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Minute,
	}, zaptest.NewLogger(t))
	p.client = mock

	err := p.Start(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no string value")
}

func TestOnChange_CalledOnRotation(t *testing.T) {
	mock := &mockSMClient{}
	mock.setOutput("value1")

	p := newAWSSecretProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Millisecond,
	}, zaptest.NewLogger(t))
	p.client = mock

	var lastValue atomic.Value
	p.OnChange(func(newValue string) {
		lastValue.Store(newValue)
	})

	err := p.Start(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	mock.setOutput("value2")

	assert.Eventually(t, func() bool {
		v := lastValue.Load()
		return v != nil && v.(string) == "value2"
	}, 5*time.Second, time.Millisecond)

	val, err := p.GetSecret(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "value2", val)
}

func TestOnChange_NotCalledWhenUnchanged(t *testing.T) {
	mock := &mockSMClient{}
	mock.setOutput("same-value")

	p := newAWSSecretProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Millisecond,
	}, zaptest.NewLogger(t))
	p.client = mock

	var callCount atomic.Int32
	p.OnChange(func(_ string) {
		callCount.Add(1)
	})

	err := p.Start(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), callCount.Load())
}

func TestRefreshError_KeepsStaleValue(t *testing.T) {
	mock := &mockSMClient{}
	mock.setOutput("good-value")

	p := newAWSSecretProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Millisecond,
	}, zaptest.NewLogger(t))
	p.client = mock

	err := p.Start(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Shutdown(context.Background()) })

	mock.setError(errors.New("transient failure"))
	time.Sleep(50 * time.Millisecond)

	val, err := p.GetSecret(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "good-value", val)
}

func TestShutdown(t *testing.T) {
	mock := &mockSMClient{}
	mock.setOutput("value")

	p := newAWSSecretProvider(&Config{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:          "us-east-1",
		RefreshInterval: time.Millisecond,
	}, zaptest.NewLogger(t))
	p.client = mock

	err := p.Start(context.Background(), nil)
	require.NoError(t, err)

	err = p.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestGetSecret_BeforeStart(t *testing.T) {
	p := newAWSSecretProvider(&Config{
		SecretARN: "arn:aws:secretsmanager:us-east-1:123:secret:test",
		Region:    "us-east-1",
	}, zaptest.NewLogger(t))

	_, err := p.GetSecret(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet loaded")
}
