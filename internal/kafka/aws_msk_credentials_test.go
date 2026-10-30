// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
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

func TestLoadAWSCredentialsProvider_ExpiryWindow(t *testing.T) {
	for _, tt := range []struct {
		name         string
		lifetime     time.Duration
		wantCalls    int32
		wantKey      string
		profile      bool
		static       bool
		stallRefresh bool
	}{
		{name: "reuse valid credentials", lifetime: 10 * time.Minute, wantCalls: 1, wantKey: "AKID1"},
		{name: "refresh near-expiry credentials", lifetime: 30 * time.Second, wantCalls: 2, wantKey: "AKID2"},
		{name: "shared profile web identity", lifetime: 10 * time.Minute, wantCalls: 1, wantKey: "AKID1", profile: true},
		{name: "static credentials take precedence", wantKey: "STATIC", static: true},
		{name: "fallback on stalled STS refresh", lifetime: 2 * time.Minute, wantCalls: 2, wantKey: "AKID1", stallRefresh: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Given a real web identity provider backed by a local STS endpoint.
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if r.Form.Get("Action") != "AssumeRoleWithWebIdentity" {
					t.Errorf("unexpected STS action: %s", r.Form.Get("Action"))
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				assert.Equal(t, "test-session", r.Form.Get("RoleSessionName"))
				call := calls.Add(1)
				if tt.stallRefresh && call > 1 {
					<-r.Context().Done()
					return
				}
				lifetime := tt.lifetime
				if call > 1 {
					lifetime = time.Hour
				}
				w.Header().Set("Content-Type", "text/xml")
				_, err := fmt.Fprintf(w, `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><AssumeRoleWithWebIdentityResult><Credentials><AccessKeyId>AKID%d</AccessKeyId><SecretAccessKey>SECRET</SecretAccessKey><SessionToken>TOKEN</SessionToken><Expiration>%s</Expiration></Credentials></AssumeRoleWithWebIdentityResult></AssumeRoleWithWebIdentityResponse>`, call, time.Now().Add(lifetime).UTC().Format(time.RFC3339))
				if err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)

			dir := t.TempDir()
			tokenFile := filepath.Join(dir, "token")
			require.NoError(t, os.WriteFile(tokenFile, []byte("test-web-identity-token"), 0o600))
			for _, name := range []string{"AWS_ACCESS_KEY_ID", "AWS_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY", "AWS_SECRET_KEY", "AWS_SESSION_TOKEN", "AWS_PROFILE", "AWS_DEFAULT_PROFILE"} {
				t.Setenv(name, "")
			}
			t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
			t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", tokenFile)
			t.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test")
			t.Setenv("AWS_ROLE_SESSION_NAME", "test-session")
			t.Setenv("AWS_ENDPOINT_URL_STS", server.URL)
			t.Setenv("AWS_IGNORE_CONFIGURED_ENDPOINT_URLS", "false")
			if tt.profile {
				t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "")
				t.Setenv("AWS_ROLE_ARN", "")
				t.Setenv("AWS_PROFILE", "msk")
				profile := fmt.Sprintf("[profile msk]\nrole_arn = arn:aws:iam::123456789012:role/test\nweb_identity_token_file = %s\nrole_session_name = test-session\n", filepath.ToSlash(tokenFile))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "config"), []byte(profile), 0o600))
			}
			if tt.static {
				t.Setenv("AWS_ACCESS_KEY_ID", "STATIC")
				t.Setenv("AWS_SECRET_ACCESS_KEY", "SECRET")
			}
			provider, err := loadAWSCredentialsProvider(t.Context(), "us-west-2", zap.NewNop())
			require.NoError(t, err)
			assert.Equal(t, !tt.static, aws.IsCredentialsProvider(provider, &awsMSKRefreshProvider{}))
			other, err := loadAWSCredentialsProvider(t.Context(), "us-west-2", zap.NewNop())
			require.NoError(t, err)
			assert.NotSame(t, provider, other, "each Kafka client must own its credential cache")
			_, err = provider.Retrieve(t.Context())
			require.NoError(t, err)

			// When credentials are requested again without waiting for expiration.
			started := time.Now()
			creds, err := provider.Retrieve(t.Context())

			// Then only credentials inside the expiry window are refreshed.
			require.NoError(t, err)
			require.Equal(t, tt.wantKey, creds.AccessKeyID)
			require.Equal(t, tt.wantCalls, calls.Load())
			if tt.stallRefresh {
				elapsed := time.Since(started)
				assert.GreaterOrEqual(t, elapsed, awsMSKCachedRefreshTimeout)
				assert.Less(t, elapsed, 2*awsMSKCachedRefreshTimeout, "the SDK HTTP refresh must use the short budget")
			}
		})
	}
}

func TestAWSMSKCredentialCacheJitter(t *testing.T) {
	actualExpiry := time.Now().Add(time.Hour)
	provider := newAWSMSKRefreshProvider(aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET", CanExpire: true, Expires: actualExpiry}, nil
	}), zap.NewNop())
	cache := aws.NewCredentialsCache(provider, configureAWSMSKCache)
	creds, err := cache.Retrieve(t.Context())
	require.NoError(t, err)
	earlyBy := actualExpiry.Sub(creds.Expires)
	assert.GreaterOrEqual(t, earlyBy, awsMSKExpiryWindow/2)
	assert.LessOrEqual(t, earlyBy, awsMSKExpiryWindow)
}

func TestAWSMSKCredentialCacheRefreshLifecycle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		actualExpiry := time.Now().Add(time.Hour)
		var failing atomic.Bool
		var calls atomic.Int32
		refreshErr := errors.New("STS unavailable")
		logs, observed := observer.New(zap.DebugLevel)
		provider := newAWSMSKRefreshProvider(aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			call := calls.Add(1)
			if failing.Load() {
				return aws.Credentials{}, refreshErr
			}
			return aws.Credentials{AccessKeyID: fmt.Sprintf("AKID%d", call), SecretAccessKey: "SECRET", SessionToken: "SESSION_TOKEN", CanExpire: true, Expires: time.Now().Add(time.Hour)}, nil
		}), zap.New(logs))
		provider.retryDelay = func() time.Duration { return 45 * time.Second }
		cache := aws.NewCredentialsCache(provider, configureAWSMSKCache)
		originalLoader := loadAWSCredentialsProvider
		t.Cleanup(func() { loadAWSCredentialsProvider = originalLoader })
		loadAWSCredentialsProvider = func(context.Context, string, *zap.Logger) (aws.CredentialsProvider, error) {
			return cache, nil
		}
		opt, err := configureKgoSASL(t.Context(), &configkafka.SASLConfig{
			Mechanism: AWSMSKIAMOAUTHBEARER,
			AWSMSK:    configkafka.AWSMSKConfig{Region: "us-west-2"},
		}, componenttest.NewNopHost(), zap.NewNop())
		require.NoError(t, err)
		cl, err := kgo.NewClient(opt)
		require.NoError(t, err)
		mechanism := cl.OptValue(kgo.SASL).([]sasl.Mechanism)[0]
		cl.Close()
		authenticate := func(wantKey string) {
			t.Helper()
			_, response, authErr := mechanism.Authenticate(t.Context(), "broker:9098")
			require.NoError(t, authErr)
			_, bearer, ok := strings.Cut(string(response), "auth=Bearer ")
			require.True(t, ok)
			signedURL, decodeErr := base64.RawURLEncoding.DecodeString(strings.TrimRight(bearer, "\x01"))
			require.NoError(t, decodeErr)
			parsed, parseErr := url.Parse(string(signedURL))
			require.NoError(t, parseErr)
			assert.True(t, strings.HasPrefix(parsed.Query().Get("X-Amz-Credential"), wantKey+"/"))
			assert.NotEmpty(t, parsed.Query().Get("X-Amz-Signature"))
		}
		creds, err := cache.Retrieve(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "AKID1", creds.AccessKeyID)
		authenticate("AKID1")
		failing.Store(true)
		time.Sleep(time.Until(creds.Expires))

		fallbackExpiry := actualExpiry.Add(-awsMSKFallbackSafetyMargin)
		for time.Now().Before(fallbackExpiry) {
			fallback, retrieveErr := cache.Retrieve(t.Context())
			require.NoError(t, retrieveErr)
			assert.Equal(t, "AKID1", fallback.AccessKeyID)
			retryIn := min(45*time.Second, time.Until(fallbackExpiry))
			assert.Equal(t, time.Now().Add(retryIn), fallback.Expires)
			before := calls.Load()
			authenticate("AKID1")
			assert.Equal(t, before, calls.Load(), "fallback must be cached until its retry deadline")
			time.Sleep(retryIn)
		}
		expired, err := cache.Retrieve(t.Context())
		require.ErrorIs(t, err, refreshErr)
		assert.False(t, expired.HasKeys(), "fallback must stop before the original AWS expiry")
		_, response, err := mechanism.Authenticate(t.Context(), "broker:9098")
		require.ErrorIs(t, err, refreshErr)
		assert.Empty(t, response, "credentials inside the safety margin must never produce an OAuth token")
		time.Sleep(time.Until(actualExpiry))
		_, response, err = mechanism.Authenticate(t.Context(), "broker:9098")
		require.ErrorIs(t, err, refreshErr)
		assert.Empty(t, response, "expired credentials must never produce an OAuth token")
		time.Sleep(time.Second)
		_, err = cache.Retrieve(t.Context())
		require.ErrorIs(t, err, refreshErr)

		failing.Store(false)
		recovered, err := cache.Retrieve(t.Context())
		require.NoError(t, err)
		assert.NotEqual(t, "AKID1", recovered.AccessKeyID)
		assert.True(t, recovered.Expires.After(time.Now()))
		authenticate(recovered.AccessKeyID)
		assert.Positive(t, observed.FilterMessage("AWS MSK credential refresh retaining still-valid credentials").Len())
		failures := observed.FilterMessage("AWS MSK credential refresh failed").All()
		require.NotEmpty(t, failures)
		for _, entry := range failures {
			assert.Equal(t, zap.WarnLevel, entry.Level)
			fields := entry.ContextMap()
			assert.Equal(t, refreshErr.Error(), fields["error"])
			assert.Contains(t, fields, "duration")
		}
		for _, entry := range observed.All() {
			assert.NotContains(t, fmt.Sprint(entry.ContextMap()), "SECRET")
			assert.NotContains(t, fmt.Sprint(entry.ContextMap()), "AKID")
			assert.NotContains(t, fmt.Sprint(entry.ContextMap()), "SESSION_TOKEN")
		}
	})
}

func TestAWSMSKCredentialCacheConcurrentRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		release := make(chan struct{})
		provider := newAWSMSKRefreshProvider(aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			calls.Add(1)
			<-release
			return aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET", CanExpire: true, Expires: time.Now().Add(time.Hour)}, nil
		}), zap.NewNop())
		cache := aws.NewCredentialsCache(provider, configureAWSMSKCache)
		var wg sync.WaitGroup
		for range 32 {
			wg.Go(func() {
				_, err := cache.Retrieve(t.Context())
				assert.NoError(t, err)
			})
		}
		synctest.Wait()
		assert.Equal(t, int32(1), calls.Load(), "the SDK must collapse concurrent refreshes")
		close(release)
		wg.Wait()
	})
}

func TestAWSMSKCredentialCacheRefreshTimeoutWithFallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		provider := newAWSMSKRefreshProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			if calls.Add(1) == 1 {
				return aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET", CanExpire: true, Expires: time.Now().Add(time.Hour)}, nil
			}
			deadline, ok := ctx.Deadline()
			assert.True(t, ok)
			assert.Equal(t, awsMSKCachedRefreshTimeout, time.Until(deadline))
			<-ctx.Done()
			return aws.Credentials{}, ctx.Err()
		}), zap.NewNop())
		cache := aws.NewCredentialsCache(provider, configureAWSMSKCache)
		originalLoader := loadAWSCredentialsProvider
		t.Cleanup(func() { loadAWSCredentialsProvider = originalLoader })
		loadAWSCredentialsProvider = func(context.Context, string, *zap.Logger) (aws.CredentialsProvider, error) {
			return cache, nil
		}
		opt, err := configureKgoSASL(t.Context(), &configkafka.SASLConfig{
			Mechanism: AWSMSKIAMOAUTHBEARER,
			AWSMSK:    configkafka.AWSMSKConfig{Region: "us-west-2"},
		}, componenttest.NewNopHost(), zap.NewNop())
		require.NoError(t, err)
		cl, err := kgo.NewClient(opt)
		require.NoError(t, err)
		mechanism := cl.OptValue(kgo.SASL).([]sasl.Mechanism)[0]
		cl.Close()
		creds, err := cache.Retrieve(t.Context())
		require.NoError(t, err)
		_, hasDeadline := t.Context().Deadline()
		require.False(t, hasDeadline, "exercise authentication without a caller deadline")
		// Cover both the first early refresh and a retry during fallback.
		for round := range 2 {
			time.Sleep(time.Until(creds.Expires))
			started := time.Now()
			var wg sync.WaitGroup
			for range 32 {
				wg.Go(func() {
					_, response, authErr := mechanism.Authenticate(t.Context(), "broker:9098")
					assert.NoError(t, authErr)
					assert.NotEmpty(t, response)
				})
			}
			wg.Wait()
			assert.Equal(t, awsMSKCachedRefreshTimeout, time.Since(started))
			assert.Equal(t, int32(round+2), calls.Load(), "concurrent authentications must share one refresh")
			creds, err = cache.Retrieve(t.Context())
			require.NoError(t, err)
			assert.Equal(t, "AKID", creds.AccessKeyID)
		}
	})
}

func TestAWSMSKCredentialCacheRefreshTimeout(t *testing.T) {
	for _, callerTimeout := range []time.Duration{0, time.Second} {
		t.Run(callerTimeout.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				finished := make(chan struct{})
				provider := newAWSMSKRefreshProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
					defer close(finished)
					deadline, ok := ctx.Deadline()
					assert.True(t, ok)
					assert.Equal(t, awsMSKRefreshTimeout, time.Until(deadline))
					<-ctx.Done()
					return aws.Credentials{}, ctx.Err()
				}), zap.NewNop())
				cache := aws.NewCredentialsCache(provider, configureAWSMSKCache)
				ctx := t.Context()
				if callerTimeout > 0 {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, callerTimeout)
					defer cancel()
				}
				started := time.Now()
				creds, err := cache.Retrieve(ctx)
				require.ErrorIs(t, err, context.DeadlineExceeded)
				assert.False(t, creds.HasKeys())
				if callerTimeout > 0 {
					assert.Equal(t, callerTimeout, time.Since(started))
					time.Sleep(awsMSKRefreshTimeout - callerTimeout)
				} else {
					assert.Equal(t, awsMSKRefreshTimeout, time.Since(started))
				}
				synctest.Wait()
				select {
				case <-finished:
				default:
					t.Fatal("the provider refresh outlived its own timeout")
				}
			})
		})
	}
}

func TestAWSMSKRefreshProviderRejectsExpiredCredentials(t *testing.T) {
	provider := newAWSMSKRefreshProvider(aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET", CanExpire: true, Expires: time.Now().Add(-time.Second)}, nil
	}), zap.NewNop())
	creds, err := aws.NewCredentialsCache(provider, configureAWSMSKCache).Retrieve(t.Context())
	require.ErrorContains(t, err, "expired credentials")
	assert.False(t, creds.HasKeys())
}

func TestAWSMSKCredentialCacheFallbackExpiresDuringRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		provider := newAWSMSKRefreshProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			if calls.Add(1) == 1 {
				return aws.Credentials{AccessKeyID: "AKID", SecretAccessKey: "SECRET", CanExpire: true, Expires: time.Now().Add(awsMSKFallbackSafetyMargin + awsMSKCachedRefreshTimeout/2)}, nil
			}
			<-ctx.Done()
			return aws.Credentials{}, ctx.Err()
		}), zap.NewNop())
		cache := aws.NewCredentialsCache(provider, configureAWSMSKCache)
		_, err := cache.Retrieve(t.Context())
		require.NoError(t, err)
		// The short refresh starts with usable credentials, but finishes inside
		// their safety margin. Recheck fallback eligibility after it completes.
		started := time.Now()
		creds, err := cache.Retrieve(t.Context())
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, awsMSKCachedRefreshTimeout, time.Since(started))
		assert.False(t, creds.HasKeys())
		assert.True(t, provider.actualExpiry.After(time.Now()), "AWS validity alone must not bypass the safety margin")
		started = time.Now()
		_, err = cache.Retrieve(t.Context())
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, awsMSKRefreshTimeout, time.Since(started), "fresh credentials now require the full refresh budget")
	})
}
