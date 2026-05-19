// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerauthextension

import (
	"context"
	"crypto/sha1" //nolint:gosec
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap/zaptest"
)

// mockClient implements secretsManagerClient for testing.
type mockClient struct {
	mu      sync.Mutex
	results []mockResult
	idx     int
}

type mockResult struct {
	secretString string
	versionID    string
	err          error
}

func (m *mockClient) GetSecretValue(_ context.Context, _ *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i := m.idx
	if i >= len(m.results) {
		i = len(m.results) - 1
	}
	m.idx++
	r := m.results[i]
	if r.err != nil {
		return nil, r.err
	}
	return &secretsmanager.GetSecretValueOutput{
		SecretString: &r.secretString,
		VersionId:    &r.versionID,
	}, nil
}

func htpasswdEntry(username, password string) string {
	h := sha1.Sum([]byte(password)) //nolint:gosec
	return fmt.Sprintf("%s:{SHA}%s", username, base64.StdEncoding.EncodeToString(h[:]))
}

func clientSecret(username, password string) string {
	b, _ := json.Marshal(map[string]string{"username": username, "password": password})
	return string(b)
}

// ---- poller tests ----

func TestPollerInitialFetch(t *testing.T) {
	defer goleak.VerifyNone(t)

	mock := &mockClient{results: []mockResult{{secretString: "secret-value", versionID: "v1"}}}
	p := &poller{client: mock, secretARN: "arn:test", refreshInterval: time.Hour, logger: zaptest.NewLogger(t)}

	var got string
	require.NoError(t, p.Start(context.Background(), func(v string) { got = v }))
	p.Shutdown()

	assert.Equal(t, "secret-value", got)
	assert.Equal(t, "v1", p.lastVersionID)
}

func TestPollerSkipsSameVersion(t *testing.T) {
	defer goleak.VerifyNone(t)

	mock := &mockClient{results: []mockResult{
		{secretString: "val", versionID: "v1"},
		{secretString: "val", versionID: "v1"},
	}}

	calls := 0
	p := &poller{client: mock, secretARN: "arn:test", refreshInterval: 10 * time.Millisecond, logger: zaptest.NewLogger(t)}
	require.NoError(t, p.Start(context.Background(), func(_ string) { calls++ }))
	time.Sleep(50 * time.Millisecond)
	p.Shutdown()

	assert.Equal(t, 1, calls)
}

func TestPollerFiresOnVersionChange(t *testing.T) {
	defer goleak.VerifyNone(t)

	mock := &mockClient{results: []mockResult{
		{secretString: "first", versionID: "v1"},
		{secretString: "second", versionID: "v2"},
	}}

	var mu sync.Mutex
	var received []string
	p := &poller{client: mock, secretARN: "arn:test", refreshInterval: 10 * time.Millisecond, logger: zaptest.NewLogger(t)}
	require.NoError(t, p.Start(context.Background(), func(v string) {
		mu.Lock()
		received = append(received, v)
		mu.Unlock()
	}))
	assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) >= 2
	}, time.Second, 5*time.Millisecond)
	p.Shutdown()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"first", "second"}, received[:2])
}

func TestPollerKeepsCredentialsOnFetchError(t *testing.T) {
	defer goleak.VerifyNone(t)

	mock := &mockClient{results: []mockResult{
		{secretString: "initial", versionID: "v1"},
		{err: errors.New("network error")},
	}}

	calls := 0
	p := &poller{client: mock, secretARN: "arn:test", refreshInterval: 10 * time.Millisecond, logger: zaptest.NewLogger(t)}
	require.NoError(t, p.Start(context.Background(), func(_ string) { calls++ }))
	time.Sleep(50 * time.Millisecond)
	p.Shutdown()

	assert.Equal(t, 1, calls)
	assert.Equal(t, "v1", p.lastVersionID)
}

func TestPollerShutdownStopsGoroutine(t *testing.T) {
	defer goleak.VerifyNone(t)

	mock := &mockClient{results: []mockResult{{secretString: "v", versionID: "v1"}}}
	p := &poller{client: mock, secretARN: "arn:test", refreshInterval: time.Hour, logger: zaptest.NewLogger(t)}
	require.NoError(t, p.Start(context.Background(), func(_ string) {}))
	p.Shutdown()
}

// ---- client auth tests ----

func TestClientAuthSwapsCredentials(t *testing.T) {
	defer goleak.VerifyNone(t)

	mock := &mockClient{results: []mockResult{
		{secretString: clientSecret("alice", "s3cr3t"), versionID: "v1"},
		{secretString: clientSecret("bob", "n3wp4ss"), versionID: "v2"},
	}}

	cfg := &Config{SecretARN: "arn:test", RefreshInterval: 10 * time.Millisecond, ClientAuth: &ClientAuthSettings{}}
	ext := newClientAuth(cfg, zaptest.NewLogger(t))
	require.NoError(t, ext.startWithClient(context.Background(), mock))

	assert.Equal(t, "alice", ext.username())
	assert.Equal(t, "s3cr3t", ext.password())

	assert.Eventually(t, func() bool { return ext.username() == "bob" }, time.Second, 5*time.Millisecond)
	assert.Equal(t, "n3wp4ss", ext.password())

	require.NoError(t, ext.Shutdown(context.Background()))
}

// captureTransport records the request it receives so tests can inspect the injected headers.
type captureTransport struct{ last *http.Request }

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.last = req
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func TestClientAuthRoundTripper(t *testing.T) {
	defer goleak.VerifyNone(t)

	mock := &mockClient{results: []mockResult{
		{secretString: clientSecret("alice", "pass"), versionID: "v1"},
	}}

	cfg := &Config{SecretARN: "arn:test", RefreshInterval: time.Hour, ClientAuth: &ClientAuthSettings{}}
	ext := newClientAuth(cfg, zaptest.NewLogger(t))
	require.NoError(t, ext.startWithClient(context.Background(), mock))

	capture := &captureTransport{}
	rt, err := ext.RoundTripper(capture)
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	_, _ = rt.RoundTrip(req)
	require.NotNil(t, capture.last)
	assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("alice:pass")), capture.last.Header.Get("Authorization"))

	require.NoError(t, ext.Shutdown(context.Background()))
}

func TestClientAuthCustomKeys(t *testing.T) {
	defer goleak.VerifyNone(t)

	secret, _ := json.Marshal(map[string]string{"user": "alice", "pass": "s3cr3t"})
	mock := &mockClient{results: []mockResult{{secretString: string(secret), versionID: "v1"}}}

	cfg := &Config{
		SecretARN:       "arn:test",
		RefreshInterval: time.Hour,
		ClientAuth:      &ClientAuthSettings{UsernameKey: "user", PasswordKey: "pass"},
	}
	ext := newClientAuth(cfg, zaptest.NewLogger(t))
	require.NoError(t, ext.startWithClient(context.Background(), mock))

	assert.Equal(t, "alice", ext.username())
	assert.Equal(t, "s3cr3t", ext.password())

	require.NoError(t, ext.Shutdown(context.Background()))
}

// ---- server auth tests ----

func TestServerAuthSwapsMatchFunc(t *testing.T) {
	defer goleak.VerifyNone(t)

	htV1 := htpasswdEntry("alice", "pass1")
	htV2 := htpasswdEntry("bob", "pass2")

	mock := &mockClient{results: []mockResult{
		{secretString: htV1, versionID: "v1"},
		{secretString: htV2, versionID: "v2"},
	}}

	cfg := &Config{SecretARN: "arn:test", RefreshInterval: 10 * time.Millisecond, Htpasswd: &HtpasswdSettings{}}
	ext := newServerAuth(cfg, zaptest.NewLogger(t))
	require.NoError(t, ext.startWithClient(context.Background(), mock))

	assert.Eventually(t, func() bool {
		fn := ext.matchFunc.Load()
		return fn != nil && (*fn)("alice", "pass1")
	}, time.Second, 5*time.Millisecond)
	assert.False(t, (*ext.matchFunc.Load())("bob", "pass2"))

	assert.Eventually(t, func() bool {
		fn := ext.matchFunc.Load()
		return fn != nil && (*fn)("bob", "pass2")
	}, time.Second, 5*time.Millisecond)
	assert.False(t, (*ext.matchFunc.Load())("alice", "pass1"))

	require.NoError(t, ext.Shutdown(context.Background()))
}

func TestServerAuthAuthenticate(t *testing.T) {
	defer goleak.VerifyNone(t)

	mock := &mockClient{results: []mockResult{
		{secretString: htpasswdEntry("alice", "pass1"), versionID: "v1"},
	}}

	cfg := &Config{SecretARN: "arn:test", RefreshInterval: time.Hour, Htpasswd: &HtpasswdSettings{}}
	ext := newServerAuth(cfg, zaptest.NewLogger(t))
	require.NoError(t, ext.startWithClient(context.Background(), mock))

	encoded := base64.StdEncoding.EncodeToString([]byte("alice:pass1"))
	ctx, err := ext.Authenticate(context.Background(), map[string][]string{
		"Authorization": {"Basic " + encoded},
	})
	require.NoError(t, err)
	assert.NotNil(t, ctx)

	_, err = ext.Authenticate(context.Background(), map[string][]string{
		"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte("alice:wrong"))},
	})
	assert.ErrorIs(t, err, errInvalidCredentials)

	require.NoError(t, ext.Shutdown(context.Background()))
}

func TestServerAuthValueKey(t *testing.T) {
	defer goleak.VerifyNone(t)

	entry := htpasswdEntry("alice", "pass1")
	secret, _ := json.Marshal(map[string]string{"htpasswd": entry})
	mock := &mockClient{results: []mockResult{{secretString: string(secret), versionID: "v1"}}}

	cfg := &Config{SecretARN: "arn:test", RefreshInterval: time.Hour, Htpasswd: &HtpasswdSettings{ValueKey: "htpasswd"}}
	ext := newServerAuth(cfg, zaptest.NewLogger(t))
	require.NoError(t, ext.startWithClient(context.Background(), mock))

	encoded := base64.StdEncoding.EncodeToString([]byte("alice:pass1"))
	_, err := ext.Authenticate(context.Background(), map[string][]string{
		"Authorization": {"Basic " + encoded},
	})
	require.NoError(t, err)

	require.NoError(t, ext.Shutdown(context.Background()))
}

// ---- config validation tests ----

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid client auth",
			cfg:  Config{SecretARN: "arn:test", RefreshInterval: 30 * time.Second, ClientAuth: &ClientAuthSettings{}},
		},
		{
			name: "valid htpasswd",
			cfg:  Config{SecretARN: "arn:test", RefreshInterval: 30 * time.Second, Htpasswd: &HtpasswdSettings{}},
		},
		{
			name:    "missing secret_arn",
			cfg:     Config{RefreshInterval: 30 * time.Second, ClientAuth: &ClientAuthSettings{}},
			wantErr: true,
		},
		{
			name:    "zero refresh_interval",
			cfg:     Config{SecretARN: "arn:test", ClientAuth: &ClientAuthSettings{}},
			wantErr: true,
		},
		{
			name:    "both modes set",
			cfg:     Config{SecretARN: "arn:test", RefreshInterval: 30 * time.Second, Htpasswd: &HtpasswdSettings{}, ClientAuth: &ClientAuthSettings{}},
			wantErr: true,
		},
		{
			name:    "no mode set",
			cfg:     Config{SecretARN: "arn:test", RefreshInterval: 30 * time.Second},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
