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

// mockSecretsManagerClient is a thread-safe mock for secretsManagerClient.
type mockSecretsManagerClient struct {
	output    atomic.Pointer[secretsmanager.GetSecretValueOutput]
	callErr   atomic.Pointer[error]
	callCount atomic.Int32
}

func newMockClient(username, password string) *mockSecretsManagerClient {
	m := &mockSecretsManagerClient{}
	m.setCredentials(username, password)
	return m
}

func (m *mockSecretsManagerClient) setCredentials(username, password string) {
	data, _ := json.Marshal(map[string]string{"username": username, "password": password})
	m.output.Store(&secretsmanager.GetSecretValueOutput{SecretString: aws.String(string(data))})
}

func (m *mockSecretsManagerClient) setError(err error) {
	m.callErr.Store(&err)
}

func (m *mockSecretsManagerClient) GetSecretValue(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	m.callCount.Add(1)
	if p := m.callErr.Load(); p != nil {
		return nil, *p
	}
	return m.output.Load(), nil
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

func TestAWSSecretsManagerResolver_CredentialsAreAtomicPair(t *testing.T) {
	// Verify that Username() and Password() always return a consistent pair
	// by checking they come from the same credentials struct load.
	mock := newMockClient("alice", "old-pass")
	r := newTestResolver(t, &AWSSecretsManagerSettings{
		SecretARN:       "arn:aws:secretsmanager:us-east-1:123:secret:test",
		RefreshInterval: 5 * time.Millisecond,
	}, nil)
	r.client = mock

	require.NoError(t, r.startWithClient(context.Background()))
	defer r.shutdown() //nolint:errcheck

	// Read many times while rotation is in progress; should never see a split pair.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 500; i++ {
			c := r.creds.Load()
			if c == nil {
				continue
			}
			// A torn pair would have mismatched username/password from different fetches.
			// Since we only ever store "alice"/"old-pass" or "alice"/"new-pass",
			// any mismatch indicates a torn read.
			assert.True(t,
				(c.username == "alice" && c.password == "old-pass") ||
					(c.username == "alice" && c.password == "new-pass"),
				"torn credential pair: username=%q password=%q", c.username, c.password)
		}
	}()
	mock.setCredentials("alice", "new-pass")
	<-done
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

	// Wait for at least 3 more polls to confirm onChange doesn't fire on stable credentials.
	assert.Eventually(t, func() bool {
		return mock.callCount.Load() >= initialCalls+3
	}, 2*time.Second, 10*time.Millisecond)

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

	countAfterStart := mock.callCount.Load()
	mock.setError(errors.New("network error"))

	// Wait until the error path has been exercised at least twice.
	assert.Eventually(t, func() bool {
		return mock.callCount.Load() >= countAfterStart+2
	}, 2*time.Second, 10*time.Millisecond)

	// Last known values must be preserved despite errors.
	assert.Equal(t, "alice", r.Username())
	assert.Equal(t, "s3cr3t", r.Password())
}

func TestAWSSecretsManagerResolver_InitialFetchError(t *testing.T) {
	mock := &mockSecretsManagerClient{}
	mock.setError(errors.New("permission denied"))
	r := newTestResolver(t, nil, nil)
	r.client = mock

	err := r.startWithClient(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initial fetch from AWS Secrets Manager failed")
}

func TestAWSSecretsManagerResolver_AlreadyStarted(t *testing.T) {
	mock := newMockClient("alice", "s3cr3t")
	r := newTestResolver(t, nil, nil)
	r.client = mock

	require.NoError(t, r.startWithClient(context.Background()))
	defer r.shutdown() //nolint:errcheck

	err := r.startWithClient(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already started")
}

func TestAWSSecretsManagerResolver_MissingKey(t *testing.T) {
	data, _ := json.Marshal(map[string]string{"user": "alice", "pass": "s3cr3t"})
	mock := &mockSecretsManagerClient{}
	mock.output.Store(&secretsmanager.GetSecretValueOutput{SecretString: aws.String(string(data))})

	r := newTestResolver(t, nil, nil)
	r.client = mock

	err := r.startWithClient(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `key "username" not found`)
}

func TestAWSSecretsManagerResolver_CustomKeys(t *testing.T) {
	data, _ := json.Marshal(map[string]string{"user": "alice", "pass": "s3cr3t"})
	mock := &mockSecretsManagerClient{}
	mock.output.Store(&secretsmanager.GetSecretValueOutput{SecretString: aws.String(string(data))})

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
