// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestResolveRegion_PriorityOrder(t *testing.T) {
	logger := zap.NewNop()

	t.Run("ConfigRegionWinsOverEnv", func(t *testing.T) {
		t.Setenv("AWS_REGION", "env-region")
		s := &AWSSessionSettings{Region: "config-region"}
		got, err := resolveRegion(context.Background(), logger, s, nil)
		require.NoError(t, err)
		assert.Equal(t, "config-region", got)
	})

	t.Run("EnvWinsWhenConfigEmpty", func(t *testing.T) {
		t.Setenv("AWS_REGION", "env-region")
		s := &AWSSessionSettings{}
		got, err := resolveRegion(context.Background(), logger, s, nil)
		require.NoError(t, err)
		assert.Equal(t, "env-region", got)
	})

	t.Run("LocalModeSkipsIMDS", func(t *testing.T) {
		t.Setenv("AWS_REGION", "")
		s := &AWSSessionSettings{LocalMode: true}
		got, err := resolveRegion(context.Background(), logger, s, nil)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestGetAWSConfig_NoRegionResolvable(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	cfg, err := GetAWSConfig(context.Background(), zap.NewNop(), &AWSSessionSettings{
		LocalMode:             true,
		NumberOfWorkers:       8,
		RequestTimeoutSeconds: 30,
	})
	assert.Error(t, err)
	assert.Equal(t, aws.Config{}, cfg)
}

// staticCredsEnv installs static AWS credentials via env vars and scrubs
// any inherited shared-config / profile env so config.LoadDefaultConfig
// resolves deterministically without consulting IMDS or the EC2 instance
// role.
func staticCredsEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAIOSFODNN7EXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "")
	t.Setenv("AWS_CONFIG_FILE", "")
	t.Setenv("AWS_SDK_LOAD_CONFIG", "")
}

func TestGetAWSConfig_ExplicitRegion(t *testing.T) {
	staticCredsEnv(t)

	cfg, err := GetAWSConfig(context.Background(), zap.NewNop(), &AWSSessionSettings{
		Region:                "us-east-1",
		NumberOfWorkers:       8,
		RequestTimeoutSeconds: 30,
		MaxRetries:            2,
	})
	require.NoError(t, err)
	assert.Equal(t, "us-east-1", cfg.Region)
	// MaxRetries: 2 → RetryMaxAttempts: 3 (v2 counts the initial attempt).
	assert.Equal(t, 3, cfg.RetryMaxAttempts)
}

func TestGetAWSConfig_EndpointThreaded(t *testing.T) {
	staticCredsEnv(t)

	cfg, err := GetAWSConfig(context.Background(), zap.NewNop(), &AWSSessionSettings{
		Region:                "us-east-1",
		Endpoint:              "https://example-endpoint.local",
		NumberOfWorkers:       8,
		RequestTimeoutSeconds: 30,
	})
	require.NoError(t, err)
	require.NotNil(t, cfg.BaseEndpoint)
	assert.Equal(t, "https://example-endpoint.local", *cfg.BaseEndpoint)
}

func TestGetAWSConfig_RetryMaxAttempts(t *testing.T) {
	// settings.MaxRetries=N must produce cfg.RetryMaxAttempts=N+1.
	staticCredsEnv(t)

	tests := []struct {
		maxRetries          int
		wantRetryMaxAttempt int
	}{
		{0, 1},
		{1, 2},
		{2, 3},
		{5, 6},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("MaxRetries=%d", tc.maxRetries), func(t *testing.T) {
			cfg, err := GetAWSConfig(context.Background(), zap.NewNop(), &AWSSessionSettings{
				Region:                "us-east-1",
				NumberOfWorkers:       8,
				RequestTimeoutSeconds: 30,
				MaxRetries:            tc.maxRetries,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.wantRetryMaxAttempt, cfg.RetryMaxAttempts)
		})
	}
}

func TestGetAWSConfig_DoesNotMutateSettings(t *testing.T) {
	// GetAWSConfig must not write back to the caller's *AWSSessionSettings.
	staticCredsEnv(t)

	settings := &AWSSessionSettings{
		Region:                "us-east-1",
		NumberOfWorkers:       8,
		RequestTimeoutSeconds: 30,
		MaxRetries:            2,
		Profile:               "test-profile",
		SharedCredentialsFile: []string{"/tmp/test-credentials"},
		CertificateFilePath:   "testdata/public_amazon_cert.pem",
		IMDSRetries:           3,
		LocalMode:             false,
		ResourceARN:           "arn:aws:resource",
	}
	before := *settings

	_, err := GetAWSConfig(context.Background(), zap.NewNop(), settings)
	require.NoError(t, err)

	assert.Equal(t, before, *settings)
}
