// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"

import (
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

const (
	defaultExpiryWindow = time.Minute * 10
)

type RefreshableSharedCredentialsProvider struct {
	credentials.Expiry
	sharedCredentialsProvider *credentials.SharedCredentialsProvider

	// Retrival frequency, if the value is 15 minutes, the credentials will be retrieved every 15 minutes.
	ExpiryWindow time.Duration
}

// Retrieve reads and extracts the shared credentials from the current
// users home directory.
func (p *RefreshableSharedCredentialsProvider) Retrieve() (credentials.Value, error) {
	if p.ExpiryWindow == 0 {
		p.ExpiryWindow = defaultExpiryWindow
	}
	p.SetExpiration(time.Now().Add(p.ExpiryWindow), 0)
	creds, err := p.sharedCredentialsProvider.Retrieve()
	creds.ProviderName = "RefreshableSharedCredentialsProvider"

	return creds, err
}
