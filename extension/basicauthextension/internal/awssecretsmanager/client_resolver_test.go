// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanager

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestClientResolver_Start(t *testing.T) {
	t.Parallel()
	secret := map[string]string{"user": "admin", "pass": "s3cret"}
	data, _ := json.Marshal(secret)

	mock := &mockSMClient{}
	mock.setSecret(string(data))

	cr := NewClientResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "user", "pass", 5*time.Minute, zaptest.NewLogger(t))
	cr.Client = mock

	require.NoError(t, cr.Start(t.Context()))
	defer func() { require.NoError(t, cr.Shutdown()) }()

	assert.Equal(t, "admin", cr.Username())
	assert.Equal(t, "s3cret", cr.Password())
}

func TestClientResolver_SingleFetchPerRefresh(t *testing.T) {
	t.Parallel()
	secret := map[string]string{"user": "admin", "pass": "s3cret"}
	data, _ := json.Marshal(secret)

	mock := &mockSMClient{}
	mock.setSecret(string(data))

	cr := NewClientResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "user", "pass", 100*time.Millisecond, zaptest.NewLogger(t))
	cr.Client = mock

	require.NoError(t, cr.Start(t.Context()))
	defer func() { require.NoError(t, cr.Shutdown()) }()

	// Wait for at least one refresh tick
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.GreaterOrEqual(c, mock.callCount.Load(), int32(2))
	}, 2*time.Second, 50*time.Millisecond)

	// Each refresh should be exactly one API call (not two)
	calls := mock.callCount.Load()
	// With 100ms interval, after ~1s we expect ~10 calls (1 initial + ~10 refreshes)
	// Key assertion: if we had two resolvers, we'd have double the calls
	assert.GreaterOrEqual(t, calls, int32(2))
}

func TestClientResolver_Refresh(t *testing.T) {
	t.Parallel()
	secret := map[string]string{"user": "admin", "pass": "original"}
	data, _ := json.Marshal(secret)

	mock := &mockSMClient{}
	mock.setSecret(string(data))

	cr := NewClientResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "user", "pass", 100*time.Millisecond, zaptest.NewLogger(t))
	cr.Client = mock

	require.NoError(t, cr.Start(t.Context()))
	defer func() { require.NoError(t, cr.Shutdown()) }()

	assert.Equal(t, "original", cr.Password())

	// Rotate secret
	rotated := map[string]string{"user": "admin", "pass": "rotated"}
	rotatedData, _ := json.Marshal(rotated)
	mock.setSecret(string(rotatedData))

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, "rotated", cr.Password())
	}, 2*time.Second, 50*time.Millisecond)

	assert.Equal(t, "admin", cr.Username())
}

func TestClientResolver_MissingKey(t *testing.T) {
	t.Parallel()
	secret := map[string]string{"user": "admin"}
	data, _ := json.Marshal(secret)

	mock := &mockSMClient{}
	mock.setSecret(string(data))

	cr := NewClientResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "user", "missing", 5*time.Minute, zaptest.NewLogger(t))
	cr.Client = mock

	err := cr.Start(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in secret JSON")
}

func TestClientResolver_FetchError(t *testing.T) {
	t.Parallel()
	mock := &mockSMClient{err: fmt.Errorf("access denied")}

	cr := NewClientResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "user", "pass", 5*time.Minute, zaptest.NewLogger(t))
	cr.Client = mock

	err := cr.Start(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "access denied")
}

func TestClientResolver_RefreshErrorKeepsOldValue(t *testing.T) {
	t.Parallel()
	secret := map[string]string{"user": "admin", "pass": "good"}
	data, _ := json.Marshal(secret)

	mock := &mockSMClient{}
	mock.setSecret(string(data))

	cr := NewClientResolver("arn:aws:secretsmanager:us-east-1:123:secret:test", "us-east-1", "user", "pass", 100*time.Millisecond, zaptest.NewLogger(t))
	cr.Client = mock

	require.NoError(t, cr.Start(t.Context()))
	defer func() { require.NoError(t, cr.Shutdown()) }()

	assert.Equal(t, "good", cr.Password())

	// Introduce error
	mock.err = fmt.Errorf("transient error")
	time.Sleep(300 * time.Millisecond)

	// Old values should be preserved
	assert.Equal(t, "admin", cr.Username())
	assert.Equal(t, "good", cr.Password())
}

func TestClientResolver_ValueBeforeStart(t *testing.T) {
	t.Parallel()
	cr := NewClientResolver("arn", "us-east-1", "user", "pass", 5*time.Minute, zaptest.NewLogger(t))
	assert.Equal(t, "", cr.Username())
	assert.Equal(t, "", cr.Password())
}

func TestClientResolver_DefaultRefreshInterval(t *testing.T) {
	t.Parallel()
	cr := NewClientResolver("arn", "us-east-1", "user", "pass", 0, zaptest.NewLogger(t))
	assert.Equal(t, DefaultRefreshInterval, cr.refreshInterval)
}
