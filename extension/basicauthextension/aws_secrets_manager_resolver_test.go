// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type mockSecretsManagerClient struct {
	mu       atomic.Pointer[secretsmanager.GetSecretValueOutput]
	callErr  error
	callCount atomic.Int32
}

func newMockClient(username, password string) *mockSecretsManagerClient {
	m := &mockSecretsManagerClient{}
	m.setCredentials(username, password)
	return m
}

func (m *mockSecretsManagerClient) setCredentials(username, password string) {
	data, _ := json.Marshal(map[string]string{"username": username, "password": password})
	secret := aws.String(string(data))
	m.mu.Store(&secretsmanager.GetSecretValueOutput{SecretString: secret})
}

func (m *mockSecretsManagerClient) GetSecretValue(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	m.callCount.Add(1)
	if m.callErr != nil {
		return nil, m.callErr
	}
	return m.mu.Load(), nil
}

func newTestResolver(t *testing.T, cfg *AWSSecretsManagerSettings, onChange func()) *awsSecretsManagerResolver {
	t.Helper()
	if cfg == nil {
		cfg = &AWSSecretsManagerSettings{SecretARN: "arn:aws:secretsmanager:us-east-1:123:secret:test"}
	}
	return newAWSSecretsManagerResolver(cfg, zaptest.NewLogger(t), onChange)
}

func TestAWSSecretsManagerResolver_InitialFetch(t *testing.T) {
	mock := newMockClient("alice", "s3cr3t")
	r := newTestResolver(t, nil, nil)
	r.client = mock

	require.NoError(t, r.startWithClient(context.Background()))
	defer r.shutdown() //nolint:errcheck

	assert.Equal(t, "alice", r.Username())
	assert.Equal(t, "s3cr3t", r.Password())
}

func TestAWSSecretsManagerResolver_RotationDetected(t *testing.T) {
	mock := newMockClient("alice", "old-pass")
	var onChangeCalled atomic.Int32
	onChange := func() { onChangeCalled.Add(1) }

	r := newTestResolver(t, &AWSSecretsManagerSettings{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		RefreshInterval: 10 * time.Millisecond,
	}, onChange)
	r.client = mock

	require.NoError(t, r.startWithClient(context.Background()))
	defer r.shutdown() //nolint:errcheck

	// onChange fired once on initial fetch
	assert.Equal(t, int32(1), onChangeCalled.Load())

	mock.setCredentials("alice", "new-pass")

	assert.Eventually(t, func() bool {
		return r.Password() == "new-pass"
	}, 2*time.Second, 10*time.Millisecond)

	assert.Greater(t, onChangeCalled.Load(), int32(1))
}

func TestAWSSecretsManagerResolver_NoOnChangeWhenUnchanged(t *testing.T) {
	mock := newMockClient("alice", "s3cr3t")
	var onChangeCalled atomic.Int32
	onChange := func() { onChangeCalled.Add(1) }

	r := newTestResolver(t, &AWSSecretsManagerSettings{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		RefreshInterval: 10 * time.Millisecond,
	}, onChange)
	r.client = mock

	require.NoError(t, r.startWithClient(context.Background()))
	defer r.shutdown() //nolint:errcheck

	initialCalls := onChangeCalled.Load()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, initialCalls, onChangeCalled.Load(), "onChange should not fire when credentials are unchanged")
}

func TestAWSSecretsManagerResolver_PollErrorKeepsLastValue(t *testing.T) {
	mock := newMockClient("alice", "s3cr3t")

	r := newTestResolver(t, &AWSSecretsManagerSettings{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		RefreshInterval: 10 * time.Millisecond,
	}, nil)
	r.client = mock

	require.NoError(t, r.startWithClient(context.Background()))
	defer r.shutdown() //nolint:errcheck

	// inject error on subsequent calls
	mock.callErr = errors.New("network error")
	time.Sleep(50 * time.Millisecond)

	// last known values must be preserved
	assert.Equal(t, "alice", r.Username())
	assert.Equal(t, "s3cr3t", r.Password())
}

func TestAWSSecretsManagerResolver_InitialFetchError(t *testing.T) {
	mock := &mockSecretsManagerClient{callErr: errors.New("permission denied")}
	r := newTestResolver(t, nil, nil)
	r.client = mock

	err := r.startWithClient(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initial fetch from AWS Secrets Manager failed")
}

func TestAWSSecretsManagerResolver_MissingKey(t *testing.T) {
	data, _ := json.Marshal(map[string]string{"user": "alice", "pass": "s3cr3t"})
	mock := &mockSecretsManagerClient{}
	mock.mu.Store(&secretsmanager.GetSecretValueOutput{SecretString: aws.String(string(data))})

	r := newTestResolver(t, nil, nil)
	r.client = mock

	err := r.startWithClient(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `key "username" not found`)
}

func TestAWSSecretsManagerResolver_CustomKeys(t *testing.T) {
	data, _ := json.Marshal(map[string]string{"user": "alice", "pass": "s3cr3t"})
	mock := &mockSecretsManagerClient{}
	mock.mu.Store(&secretsmanager.GetSecretValueOutput{SecretString: aws.String(string(data))})

	r := newTestResolver(t, &AWSSecretsManagerSettings{
		SecretARN:   "arn:aws:secretsmanager:us-east-1:123:secret:test",
		UsernameKey: "user",
		PasswordKey: "pass",
	}, nil)
	r.client = mock

	require.NoError(t, r.startWithClient(context.Background()))
	defer r.shutdown() //nolint:errcheck

	assert.Equal(t, "alice", r.Username())
	assert.Equal(t, "s3cr3t", r.Password())
}

func TestAWSSecretsManagerResolver_ShutdownStopsPolling(t *testing.T) {
	mock := newMockClient("alice", "s3cr3t")

	r := newTestResolver(t, &AWSSecretsManagerSettings{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		RefreshInterval: 5 * time.Millisecond,
	}, nil)
	r.client = mock

	require.NoError(t, r.startWithClient(context.Background()))
	require.NoError(t, r.shutdown())

	countAfterShutdown := mock.callCount.Load()
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, countAfterShutdown, mock.callCount.Load(), "no calls should occur after shutdown")
}
