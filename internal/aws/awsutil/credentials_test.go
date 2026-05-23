// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"testing"

	override "github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overrideChainContribution mirrors the inner double loop in
// getCredentialProviderChain so chain-length tests can assert deltas
// against whatever override-chain state the process happens to be in.
// override/aws's singleton has no public reset, so this is the cleanest
// way to write tests that don't depend on prior init() state.
func overrideChainContribution(files []string) int {
	n := 0
	for _, factory := range override.GetCredentialsChainOverride().GetCredentialsChain() {
		for _, file := range files {
			if factory(file) != nil {
				n++
			}
		}
	}
	return n
}

func TestGetCredentialProviderChain_Empty(t *testing.T) {
	// Independent of override state because SharedCredentialsFile is empty.
	cfg := &AWSSessionSettings{}
	chain := getCredentialProviderChain(cfg)
	assert.Empty(t, chain)
}

func TestGetCredentialProviderChain_ProfileOnly(t *testing.T) {
	// One entry: the profile-only fallback. Independent of override state
	// because SharedCredentialsFile is empty.
	cfg := &AWSSessionSettings{Profile: "myprofile"}
	chain := getCredentialProviderChain(cfg)
	require.Len(t, chain, 1)
}

func TestGetCredentialProviderChain_SharedCredentialsFiles(t *testing.T) {
	files := []string{"/tmp/file1", "/tmp/file2"}
	cfg := &AWSSessionSettings{
		Profile:               "myprofile",
		SharedCredentialsFile: files,
	}
	chain := getCredentialProviderChain(cfg)
	want := overrideChainContribution(files) + len(files)
	require.Len(t, chain, want)
}

func TestGetCredentialProviderChain_FilesWithoutProfile(t *testing.T) {
	files := []string{"/tmp/file1"}
	cfg := &AWSSessionSettings{SharedCredentialsFile: files}
	chain := getCredentialProviderChain(cfg)
	want := overrideChainContribution(files) + len(files)
	require.Len(t, chain, want)
}

func TestGetRootCredentials_FirstNonNil(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		assert.Nil(t, getRootCredentials(&AWSSessionSettings{}))
	})

	t.Run("ProfileSet", func(t *testing.T) {
		got := getRootCredentials(&AWSSessionSettings{Profile: "p"})
		assert.NotNil(t, got)
	})
}
