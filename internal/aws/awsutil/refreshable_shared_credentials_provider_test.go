// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/stretchr/testify/assert"
)

func TestSharedCredentialsProviderExpiryWindowIsExpired(t *testing.T) {
	tmpFile, _ := os.CreateTemp(os.TempDir(), "credential")
	defer os.Remove(tmpFile.Name())
	bytes, _ := os.ReadFile("./testdata/credential_original")
	_ = os.WriteFile(tmpFile.Name(), bytes, 0o600)
	p := credentials.NewCredentials(&RefreshableSharedCredentialsProvider{
		sharedCredentialsProvider: &credentials.SharedCredentialsProvider{
			Filename: tmpFile.Name(),
			Profile:  "",
		},
		ExpiryWindow: 1 * time.Second,
	})
	creds, _ := p.Get()
	assert.Equal(t, "o1rLD3ykKN09", creds.SecretAccessKey)
	time.Sleep(1 * time.Millisecond)

	assert.False(t, p.IsExpired(), "Expect creds not to be expired.")

	bytesRotate, _ := os.ReadFile("./testdata/credential_rotate")
	_ = os.WriteFile(tmpFile.Name(), bytesRotate, 0o600)

	time.Sleep(2 * time.Second)

	assert.True(t, p.IsExpired(), "Expect creds to be expired.")
	creds, _ = p.Get()
	assert.Equal(t, "o1rLDaaaccc", creds.SecretAccessKey)
	assert.False(t, p.IsExpired(), "Expect creds not to be expired.")

	time.Sleep(1 * time.Second)
	assert.True(t, p.IsExpired(), "Expect creds to be expired.")
}
