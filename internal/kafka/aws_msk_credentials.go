// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"

import (
	"context"
	"errors"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
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
	if !aws.IsCredentialsProvider(cfg.Credentials, &stscreds.WebIdentityRoleProvider{}) {
		return cfg.Credentials, nil
	}

	// Reconstruct only the SDK-selected web identity provider with its resolved
	// options, so its uncached Retrieve can be wrapped with the cache strategies.
	// This preserves the default credential chain, including profile selection
	// and the precedence of static credentials over web identity environment vars.
	provider := stscreds.NewWebIdentityRoleProvider(webIdentityOptions.Client,
		webIdentityOptions.RoleARN, webIdentityOptions.TokenRetriever,
		func(o *stscreds.WebIdentityRoleOptions) { *o = webIdentityOptions })
	return aws.NewCredentialsCache(newAWSMSKRefreshProvider(provider, logger), configureAWSMSKCache), nil
}

func configureAWSMSKCache(o *aws.CredentialsCacheOptions) {
	o.ExpiryWindow = awsMSKExpiryWindow
	o.ExpiryWindowJitterFrac = awsMSKExpiryJitterFrac
}

// awsMSKRefreshProvider leaves credential values and concurrent refreshes owned
// by the SDK cache. It retains only the original AWS expiry, before the SDK
// subtracts its early-refresh window.
type awsMSKRefreshProvider struct {
	provider aws.CredentialsProvider
	logger   *zap.Logger

	mu                   sync.Mutex
	actualExpiry         time.Time
	skipExpiryAdjustment bool
	refreshTimeout       time.Duration
	retryDelay           func() time.Duration
}

func newAWSMSKRefreshProvider(provider aws.CredentialsProvider, logger *zap.Logger) *awsMSKRefreshProvider {
	return &awsMSKRefreshProvider{
		provider:       provider,
		logger:         logger.Named("aws_msk_credentials"),
		refreshTimeout: awsMSKRefreshTimeout,
		retryDelay: func() time.Duration {
			return 30*time.Second + time.Duration(rand.Int64N(int64(30*time.Second)+1))
		},
	}
}

func (p *awsMSKRefreshProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	// CredentialsCache suppresses the caller's deadline for its shared refresh.
	// Bound the underlying provider here, including all of the SDK's STS retries.
	ctx, cancel := context.WithTimeout(ctx, p.refreshTimeout)
	defer cancel()
	started := time.Now()
	p.logger.Debug("AWS MSK credential refresh started")
	creds, err := p.provider.Retrieve(ctx)
	if err != nil {
		fields := []zap.Field{zap.Duration("duration", time.Since(started))}
		fields = append(fields, awsMSKRefreshErrorFields(err)...)
		p.logger.Warn("AWS MSK credential refresh failed", fields...)
		return aws.Credentials{}, err
	}
	if creds.CanExpire && !creds.Expires.After(time.Now()) {
		return aws.Credentials{}, errors.New("AWS MSK credential refresh returned expired credentials")
	}
	p.mu.Lock()
	p.actualExpiry = creds.Expires
	p.skipExpiryAdjustment = false
	p.mu.Unlock()
	p.logger.Debug("AWS MSK credential refresh succeeded",
		zap.Duration("duration", time.Since(started)),
		zap.Duration("credential_time_to_expiry", time.Until(creds.Expires)))
	return creds, nil
}

func awsMSKRefreshErrorFields(err error) []zap.Field {
	netErr, isNetErr := errors.AsType[net.Error](err)
	if errors.Is(err, context.DeadlineExceeded) || (isNetErr && netErr.Timeout()) {
		return []zap.Field{zap.String("error_kind", "timeout")}
	}
	if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
		code := apiErr.ErrorCode()
		// Log only recognized codes, never response messages or arbitrary codes
		// that could contain token, request, or other sensitive contents.
		switch code {
		case "AccessDenied", "AccessDeniedException", "ExpiredToken", "ExpiredTokenException", "InvalidClientTokenId",
			"IDPCommunicationError", "IDPRejectedClaim", "InvalidIdentityToken",
			"MalformedPolicyDocument", "PackedPolicyTooLarge", "RegionDisabled", "RegionDisabledException",
			"Throttling", "ThrottlingException", "RequestTimeout", "RequestTimeoutException",
			"ServiceUnavailable", "InternalFailure", "InternalError":
		default:
			code = "unknown"
		}
		return []zap.Field{zap.String("error_kind", "sts_api_error"), zap.String("aws_error_code", code)}
	}
	if _, ok := errors.AsType[*smithyhttp.RequestSendError](err); isNetErr || ok {
		return []zap.Field{zap.String("error_kind", "network")}
	}
	return []zap.Field{zap.String("error_kind", "unknown")}
}

func (p *awsMSKRefreshProvider) HandleFailToRefresh(_ context.Context, current aws.Credentials, refreshErr error) (aws.Credentials, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
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
	p.mu.Lock()
	defer p.mu.Unlock()
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
