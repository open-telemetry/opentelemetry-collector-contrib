// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestWrapAWSMSKCredentialsProviderIncompleteOptions(t *testing.T) {
	client := sts.New(sts.Options{})
	tokenRetriever := stscreds.IdentityTokenFile("test-token")
	original := aws.NewCredentialsCache(stscreds.NewWebIdentityRoleProvider(client, "test-role", tokenRetriever))
	for _, tt := range []struct {
		name    string
		options stscreds.WebIdentityRoleOptions
	}{
		{name: "missing both"},
		{name: "missing client", options: stscreds.WebIdentityRoleOptions{TokenRetriever: tokenRetriever}},
		{name: "missing token retriever", options: stscreds.WebIdentityRoleOptions{Client: client}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := wrapAWSMSKCredentialsProvider(original, tt.options, zap.NewNop())
			assert.Same(t, original, provider, "incomplete captured options must preserve the SDK provider")
		})
	}
}

func TestAWSMSKCredentialRefreshFailureLogging(t *testing.T) {
	refreshErr := errors.New("STS unavailable")
	logs, observed := observer.New(zap.DebugLevel)
	credentials := aws.Credentials{
		AccessKeyID: "access-key-do-not-log", SecretAccessKey: "secret-key-do-not-log",
		SessionToken: "session-token-do-not-log", CanExpire: true, Expires: time.Now().Add(time.Minute),
	}
	var calls int
	provider := newAWSMSKRefreshProvider(aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		calls++
		if calls == 1 {
			return credentials, nil
		}
		return aws.Credentials{}, refreshErr
	}), zap.New(logs))
	cache := aws.NewCredentialsCache(provider, configureAWSMSKCache)
	_, err := cache.Retrieve(t.Context())
	require.NoError(t, err)
	// Fallback succeeds, but the refresh failure must remain visible in logs.
	fallback, err := cache.Retrieve(t.Context())
	require.NoError(t, err)
	assert.Equal(t, credentials.AccessKeyID, fallback.AccessKeyID)
	entries := observed.FilterMessage("AWS MSK credential refresh failed").All()
	require.Len(t, entries, 1)
	assert.Equal(t, zap.WarnLevel, entries[0].Level)
	fields := entries[0].ContextMap()
	assert.Equal(t, refreshErr.Error(), fields["error"])
	assert.Contains(t, fields, "duration")
	for _, value := range []string{credentials.AccessKeyID, credentials.SecretAccessKey, credentials.SessionToken} {
		assert.NotContains(t, fmt.Sprint(observed.All()), value)
	}
}
