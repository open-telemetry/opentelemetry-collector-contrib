// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testProfile = "default"

func TestSharedCredentialsProvider_MissingProfile(t *testing.T) {
	// config.LoadSharedConfigProfile in v2 returns empty credentials
	// without erroring on a missing file or profile. Pin that behavior.
	tmp := filepath.Join(t.TempDir(), "missing")
	p := SharedCredentialsProvider{Filename: tmp, Profile: testProfile}
	creds, err := p.Retrieve(context.Background())
	require.NoError(t, err)
	assert.Empty(t, creds.AccessKeyID)
	assert.Empty(t, creds.SecretAccessKey)
}

func TestRefreshableSharedCredentialsProvider_DefaultsExpiryWindow(t *testing.T) {
	tmpFile := writeTempCredentials(t, "credential_original")
	p := RefreshableSharedCredentialsProvider{
		Provider: SharedCredentialsProvider{Filename: tmpFile, Profile: testProfile},
		// ExpiryWindow zero → defaultExpiryWindow.
	}
	got, err := p.Retrieve(context.Background())
	require.NoError(t, err)
	assert.True(t, got.CanExpire)
	expectedMin := time.Now().Add(defaultExpiryWindow - time.Minute)
	expectedMax := time.Now().Add(defaultExpiryWindow + time.Minute)
	assert.WithinRange(t, got.Expires, expectedMin, expectedMax)
}

func TestRefreshableSharedCredentialsProvider_FileRotation(t *testing.T) {
	// Write fixture 1, retrieve, rotate file, wait past expiry, retrieve
	// again, verify the rotated value comes through.
	tmpDir := t.TempDir()
	tmpFile, err := os.CreateTemp(tmpDir, "credential")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	provider := RefreshableSharedCredentialsProvider{
		Provider:     SharedCredentialsProvider{Filename: tmpFile.Name(), Profile: testProfile},
		ExpiryWindow: 500 * time.Millisecond,
	}
	cache := aws.NewCredentialsCache(provider)

	originalContent, err := os.ReadFile(filepath.Join("testdata", "credential_original"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpFile.Name(), originalContent, 0o600))

	creds, err := cache.Retrieve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "o1rLD3ykKN09originalSECRETxxxxxxxxxxxxxxxx", creds.SecretAccessKey)
	assert.False(t, creds.Expired())

	time.Sleep(100 * time.Millisecond)
	assert.False(t, creds.Expired())

	rotatedContent, err := os.ReadFile(filepath.Join("testdata", "credential_rotate"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tmpFile.Name(), rotatedContent, 0o600))

	time.Sleep(500 * time.Millisecond)
	assert.True(t, creds.Expired())

	creds, err = cache.Retrieve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "o1rLDaaacccROTATEDsecretxxxxxxxxxxxxxxxxxx", creds.SecretAccessKey)
	assert.False(t, creds.Expired())
}

// writeTempCredentials copies a fixture into a temp file and returns its path.
func writeTempCredentials(t *testing.T, fixtureName string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", fixtureName))
	require.NoError(t, err)
	tmp := filepath.Join(t.TempDir(), "credentials")
	require.NoError(t, os.WriteFile(tmp, content, 0o600))
	return tmp
}
