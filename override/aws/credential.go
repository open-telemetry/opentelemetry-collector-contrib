// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aws // import "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"

import (
	"github.com/aws/aws-sdk-go-v2/aws"
)

type (
	credentialsProvider []func(string) aws.CredentialsProvider

	// CredentialsChainOverride is a process-global extension point that
	// lets external callers register additional aws.CredentialsProvider
	// factories ahead of the default credential resolution path used by
	// internal/aws/awsutil.
	CredentialsChainOverride struct {
		credentialsProvider credentialsProvider
	}
)

var credentialsChainOverride *CredentialsChainOverride

// GetCredentialsChainOverride returns the package-level singleton.
func GetCredentialsChainOverride() *CredentialsChainOverride {
	if credentialsChainOverride == nil {
		credentialsChainOverride = &CredentialsChainOverride{
			credentialsProvider: make([]func(string) aws.CredentialsProvider, 0),
		}
	}
	return credentialsChainOverride
}

// AppendCredentialsChain registers a credentials-provider factory. The
// factory is invoked once per entry in the consumer's
// shared-credentials file list and may return nil to decline.
// Registration is expected at init() time, before any consumer reads
// the chain.
func (c *CredentialsChainOverride) AppendCredentialsChain(credentialsProvider func(string) aws.CredentialsProvider) {
	c.credentialsProvider = append(c.credentialsProvider, credentialsProvider)
}

// GetCredentialsChain returns the current override chain. The returned
// slice must not be mutated by callers.
func (c *CredentialsChainOverride) GetCredentialsChain() []func(string) aws.CredentialsProvider {
	return c.credentialsProvider
}

func init() {
	GetCredentialsChainOverride()
}
