// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"go.uber.org/zap"
)

const (
	awsMSKExpiryWindow     = 5 * time.Minute
	awsMSKExpiryJitterFrac = 0.5
	awsMSKRefreshTimeout   = 30 * time.Second
)

// loadAWSCredentialsProvider is a variable so tests can replace it.
var loadAWSCredentialsProvider = func(ctx context.Context, region string, logger *zap.Logger) (aws.CredentialsProvider, error) {
	var webIdentityOptions stscreds.WebIdentityRoleOptions
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region),
		awsconfig.WithCredentialsCacheOptions(func(o *aws.CredentialsCacheOptions) { o.ExpiryWindow = time.Minute }),
		awsconfig.WithWebIdentityRoleCredentialOptions(func(o *stscreds.WebIdentityRoleOptions) {
			webIdentityOptions = *o
		}))
	if err != nil {
		return nil, err
	}
	return wrapAWSMSKCredentialsProvider(cfg.Credentials, webIdentityOptions, logger), nil
}

func wrapAWSMSKCredentialsProvider(provider aws.CredentialsProvider, options stscreds.WebIdentityRoleOptions, logger *zap.Logger) aws.CredentialsProvider {
	// The SDK invokes the options callback before constructing the provider as
	// well as during construction. Preserve the SDK provider if a change in that
	// ordering leaves the captured client or token retriever unset.
	if !aws.IsCredentialsProvider(provider, &stscreds.WebIdentityRoleProvider{}) || options.Client == nil || options.TokenRetriever == nil {
		return provider
	}

	// Reconstruct only the SDK-selected web identity provider, so its uncached
	// Retrieve can be wrapped with the cache strategies.
	// This preserves the default credential chain, including profile selection
	// and the precedence of static credentials over web identity environment vars.
	webIdentityProvider := stscreds.NewWebIdentityRoleProvider(options.Client,
		options.RoleARN, options.TokenRetriever,
		func(o *stscreds.WebIdentityRoleOptions) { *o = options })
	return aws.NewCredentialsCache(newAWSMSKRefreshProvider(webIdentityProvider, logger), configureAWSMSKCache)
}

func configureAWSMSKCache(o *aws.CredentialsCacheOptions) {
	o.ExpiryWindow = awsMSKExpiryWindow
	o.ExpiryWindowJitterFrac = awsMSKExpiryJitterFrac
}

// awsMSKRefreshProvider leaves credential values and concurrent refreshes owned
// by the SDK cache. It retains only the original AWS expiry, before the SDK
// subtracts its early-refresh window.
// Each instance belongs to one SDK CredentialsCache, whose singleflight
// serializes Retrieve and both cache strategy hooks.
type awsMSKRefreshProvider struct {
	provider aws.CredentialsProvider
	logger   *zap.Logger

	actualExpiry         time.Time
	skipExpiryAdjustment bool
	retryDelay           func() time.Duration
}

func newAWSMSKRefreshProvider(provider aws.CredentialsProvider, logger *zap.Logger) *awsMSKRefreshProvider {
	return &awsMSKRefreshProvider{
		provider: provider,
		logger:   logger.Named("aws_msk_credentials"),
		retryDelay: func() time.Duration {
			return 30*time.Second + time.Duration(rand.Int64N(int64(30*time.Second)+1))
		},
	}
}

func (p *awsMSKRefreshProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	// CredentialsCache suppresses the caller's deadline for its shared refresh.
	// Bound the underlying provider here, including all of the SDK's STS retries.
	ctx, cancel := context.WithTimeout(ctx, awsMSKRefreshTimeout)
	defer cancel()
	started := time.Now()
	p.logger.Debug("AWS MSK credential refresh started")
	creds, err := p.provider.Retrieve(ctx)
	if err != nil {
		p.logger.Warn("AWS MSK credential refresh failed",
			zap.Duration("duration", time.Since(started)), zap.Error(err))
		return aws.Credentials{}, err
	}
	if creds.CanExpire && !creds.Expires.After(time.Now()) {
		return aws.Credentials{}, errors.New("AWS MSK credential refresh returned expired credentials")
	}
	p.actualExpiry = creds.Expires
	p.skipExpiryAdjustment = false
	p.logger.Debug("AWS MSK credential refresh succeeded",
		zap.Duration("duration", time.Since(started)),
		zap.Duration("credential_time_to_expiry", time.Until(creds.Expires)))
	return creds, nil
}

func (p *awsMSKRefreshProvider) HandleFailToRefresh(_ context.Context, current aws.Credentials, refreshErr error) (aws.Credentials, error) {
	now := time.Now()
	if !current.HasKeys() || !current.CanExpire || !p.actualExpiry.After(now) {
		return aws.Credentials{}, refreshErr
	}
	retryIn := min(p.retryDelay(), p.actualExpiry.Sub(now))
	current.Expires = now.Add(retryIn)
	// This expiry is already the next retry deadline, capped at the original
	// AWS expiry. Subtracting the SDK window again would cause a refresh storm.
	p.skipExpiryAdjustment = true
	p.logger.Warn("AWS MSK credential refresh retaining still-valid credentials",
		zap.Duration("credential_time_to_expiry", p.actualExpiry.Sub(now)),
		zap.Duration("retry_in", retryIn))
	return current, nil
}

func (p *awsMSKRefreshProvider) AdjustExpiresBy(creds aws.Credentials, adjustment time.Duration) (aws.Credentials, error) {
	if p.skipExpiryAdjustment {
		p.skipExpiryAdjustment = false
	} else if creds.CanExpire {
		creds.Expires = creds.Expires.Add(adjustment)
	}
	return creds, nil
}

var (
	_ aws.CredentialsProvider                       = (*awsMSKRefreshProvider)(nil)
	_ aws.HandleFailRefreshCredentialsCacheStrategy = (*awsMSKRefreshProvider)(nil)
	_ aws.AdjustExpiresByCredentialsCacheStrategy   = (*awsMSKRefreshProvider)(nil)
)
