// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsiamdbauthextension

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth/awsiamdbauthextension/internal/metadata"
)

// A developer's AWS profile (role chaining, web identity) would otherwise leak into the default chain
func isolateAWSConfig(t *testing.T) {
	t.Helper()
	empty := t.TempDir() + "/empty"
	for _, k := range []string{"AWS_CONFIG_FILE", "AWS_SHARED_CREDENTIALS_FILE"} {
		t.Setenv(k, empty)
	}
	for _, k := range []string{"AWS_PROFILE", "AWS_ROLE_ARN", "AWS_WEB_IDENTITY_TOKEN_FILE", "AWS_CONTAINER_CREDENTIALS_FULL_URI", "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"} {
		t.Setenv(k, "")
	}
}

func TestCreateExtensionUsesAssumedRoleCredentials(t *testing.T) {
	isolateAWSConfig(t)
	cfg := &Config{Region: "eu-central-1", AssumeRole: AssumeRole{ARN: "arn:aws:iam::123456789012:role/reader", SessionName: "postgres-insights"}}
	ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	cache, ok := ext.(*iamExtension).awsConfig.Credentials.(*aws.CredentialsCache)
	require.True(t, ok)
	require.True(t, cache.IsCredentialsProvider(&stscreds.AssumeRoleProvider{}))
}

func TestCreateExtensionWithoutAssumeRoleKeepsDefaultChain(t *testing.T) {
	isolateAWSConfig(t)
	ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(metadata.Type), &Config{Region: "eu-central-1"})
	require.NoError(t, err)
	if cache, ok := ext.(*iamExtension).awsConfig.Credentials.(*aws.CredentialsCache); ok {
		require.False(t, cache.IsCredentialsProvider(&stscreds.AssumeRoleProvider{}))
	}
}
