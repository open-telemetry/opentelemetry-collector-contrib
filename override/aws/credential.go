// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aws // import "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
import (
	"github.com/aws/aws-sdk-go/aws/credentials"
)

type (
	credentialsProvider      []func(string) credentials.Provider
	CredentialsChainOverride struct {
		credentialsProvider credentialsProvider
	}
)

var credentialsChainOverride *CredentialsChainOverride

func GetCredentialsChainOverride() *CredentialsChainOverride {
	if credentialsChainOverride == nil {
		credentialsChainOverride = &CredentialsChainOverride{credentialsProvider: make([]func(string) credentials.Provider, 0)}
	}
	return credentialsChainOverride
}

func (c *CredentialsChainOverride) AppendCredentialsChain(credentialsProvider func(string) credentials.Provider) {
	c.credentialsProvider = append(c.credentialsProvider, credentialsProvider)
}

func (c *CredentialsChainOverride) GetCredentialsChain() []func(string) credentials.Provider {
	return c.credentialsProvider
}

func init() {
	GetCredentialsChainOverride()
}
