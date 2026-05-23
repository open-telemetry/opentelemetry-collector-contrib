// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubProvider returns a factory that constructs a labeled provider.
// The label ends up in AccessKeyID and the filename in SessionToken so
// tests can identify which factory ran with which input.
func stubProvider(label string) func(string) aws.CredentialsProvider {
	return func(filename string) aws.CredentialsProvider {
		return stubCredsProvider{label: label, filename: filename}
	}
}

type stubCredsProvider struct {
	label    string
	filename string
}

func (s stubCredsProvider) Retrieve(_ context.Context) (aws.Credentials, error) {
	return aws.Credentials{
		AccessKeyID:  s.label,
		SessionToken: s.filename,
		Source:       "stub",
	}, nil
}

// resetChain clears the singleton's chain. Tests that mutate the
// singleton must call this for isolation. Reaches into the unexported
// field directly because the public API only exposes Append.
func resetChain(t *testing.T) {
	t.Helper()
	GetCredentialsChainOverride().credentialsProvider = make([]func(string) aws.CredentialsProvider, 0)
	require.Empty(t, GetCredentialsChainOverride().GetCredentialsChain())
}

func TestGetCredentialsChainOverride_Singleton(t *testing.T) {
	a := GetCredentialsChainOverride()
	b := GetCredentialsChainOverride()
	assert.Same(t, a, b)
}

func TestAppendCredentialsChain_SingleRegistrant(t *testing.T) {
	resetChain(t)
	defer resetChain(t)

	c := GetCredentialsChainOverride()
	c.AppendCredentialsChain(stubProvider("odin"))

	got := c.GetCredentialsChain()
	require.Len(t, got, 1)

	v, err := got[0]("/tmp/creds").Retrieve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "odin", v.AccessKeyID)
	assert.Equal(t, "/tmp/creds", v.SessionToken)
}

func TestAppendCredentialsChain_MultipleRegistrantsAppendInOrder(t *testing.T) {
	resetChain(t)
	defer resetChain(t)

	c := GetCredentialsChainOverride()
	c.AppendCredentialsChain(stubProvider("a"))
	assert.Len(t, c.GetCredentialsChain(), 1)

	c.AppendCredentialsChain(stubProvider("b"))
	got := c.GetCredentialsChain()
	require.Len(t, got, 2)

	ctx := context.Background()

	first, err := got[0]("file-a").Retrieve(ctx)
	require.NoError(t, err)
	assert.Equal(t, "a", first.AccessKeyID)

	second, err := got[1]("file-b").Retrieve(ctx)
	require.NoError(t, err)
	assert.Equal(t, "b", second.AccessKeyID)
}

func TestAppendCredentialsChain_FactoryMayReturnNil(t *testing.T) {
	resetChain(t)
	defer resetChain(t)

	GetCredentialsChainOverride().AppendCredentialsChain(
		func(filename string) aws.CredentialsProvider {
			if filename == "/accept/this" {
				return stubCredsProvider{label: "yes", filename: filename}
			}
			return nil
		},
	)

	got := GetCredentialsChainOverride().GetCredentialsChain()
	require.Len(t, got, 1)

	assert.NotNil(t, got[0]("/accept/this"))
	assert.Nil(t, got[0]("/reject/that"))
}
