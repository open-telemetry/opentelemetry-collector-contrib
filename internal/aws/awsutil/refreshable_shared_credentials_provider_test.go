// Copyright The OpenTelemetry Authors
// Portions of this file Copyright 2018-2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	_ = os.WriteFile(tmpFile.Name(), bytes, 0600)
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
	_ = os.WriteFile(tmpFile.Name(), bytesRotate, 0600)

	time.Sleep(2 * time.Second)

	assert.True(t, p.IsExpired(), "Expect creds to be expired.")
	creds, _ = p.Get()
	assert.Equal(t, "o1rLDaaaccc", creds.SecretAccessKey)
	assert.False(t, p.IsExpired(), "Expect creds not to be expired.")

	time.Sleep(1 * time.Second)
	assert.True(t, p.IsExpired(), "Expect creds to be expired.")
}
