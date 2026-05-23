// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

const defaultExpiryWindow = 10 * time.Minute

// RefreshableSharedCredentialsProvider stamps an expiry on credentials
// retrieved from Provider so the SDK's credentials cache will re-read
// the underlying file periodically and pick up rotated values without
// an agent restart.
type RefreshableSharedCredentialsProvider struct {
	Provider     SharedCredentialsProvider
	ExpiryWindow time.Duration // Zero means defaultExpiryWindow.
}

var _ aws.CredentialsProvider = (*RefreshableSharedCredentialsProvider)(nil)

func (p RefreshableSharedCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	creds, err := p.Provider.Retrieve(ctx)
	if err != nil {
		return aws.Credentials{}, err
	}
	window := p.ExpiryWindow
	if window == 0 {
		window = defaultExpiryWindow
	}
	creds.CanExpire = true
	creds.Expires = time.Now().Add(window)
	return creds, nil
}

// SharedCredentialsProvider loads credentials from a shared-credentials
// file and profile. An empty Filename uses the SDK's default
// shared-credentials file resolution.
type SharedCredentialsProvider struct {
	Filename string
	Profile  string
}

var _ aws.CredentialsProvider = (*SharedCredentialsProvider)(nil)

func (p SharedCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	var opts []func(*config.LoadSharedConfigOptions)
	if p.Filename != "" {
		opts = append(opts, func(o *config.LoadSharedConfigOptions) {
			o.CredentialsFiles = []string{p.Filename}
		})
	}
	sharedConfig, err := config.LoadSharedConfigProfile(ctx, p.Profile, opts...)
	if err != nil {
		return aws.Credentials{}, err
	}
	return sharedConfig.Credentials, nil
}
