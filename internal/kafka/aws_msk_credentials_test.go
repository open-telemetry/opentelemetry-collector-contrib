// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
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
		SessionToken: "session-token-do-not-log", CanExpire: true, Expires: time.Now().Add(2 * time.Minute),
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

func TestAWSMSKCredentialCacheFallbackSafetyMargin(t *testing.T) {
	for _, tt := range []struct {
		name         string
		offset       time.Duration
		timeout      time.Duration
		wantFallback bool
	}{
		{name: "just before cutoff", offset: -time.Nanosecond, timeout: awsMSKCachedRefreshTimeout, wantFallback: true},
		{name: "at cutoff", timeout: awsMSKRefreshTimeout},
		{name: "inside margin", offset: time.Second, timeout: awsMSKRefreshTimeout},
		{name: "actual expiry", offset: awsMSKFallbackSafetyMargin, timeout: awsMSKRefreshTimeout},
		{name: "after expiry", offset: awsMSKFallbackSafetyMargin + time.Second, timeout: awsMSKRefreshTimeout},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				actualExpiry := time.Now().Add(time.Hour)
				cutoff := actualExpiry.Add(-awsMSKFallbackSafetyMargin)
				refreshErr := errors.New("STS unavailable")
				var calls int
				provider := newAWSMSKRefreshProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
					calls++
					if calls == 1 {
						return aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET", CanExpire: true, Expires: actualExpiry}, nil
					}
					deadline, ok := ctx.Deadline()
					assert.True(t, ok)
					assert.Equal(t, tt.timeout, time.Until(deadline), "timeout and fallback must use the same cutoff")
					return aws.Credentials{}, refreshErr
				}), zap.NewNop())
				cache := aws.NewCredentialsCache(provider, configureAWSMSKCache)
				_, err := cache.Retrieve(t.Context())
				require.NoError(t, err)
				time.Sleep(time.Until(cutoff.Add(tt.offset)))
				creds, err := cache.Retrieve(t.Context())
				if tt.wantFallback {
					require.NoError(t, err)
					assert.Equal(t, "AKID", creds.AccessKeyID)
					assert.Equal(t, cutoff, creds.Expires, "fallback cache deadline must not enter the safety margin")
				} else {
					require.ErrorIs(t, err, refreshErr)
					assert.False(t, creds.HasKeys())
				}
				assert.Equal(t, actualExpiry, provider.actualExpiry, "the original AWS expiry must remain unchanged")
			})
		})
	}
}
