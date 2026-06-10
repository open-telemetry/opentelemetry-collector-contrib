// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package awssecretsmanager provides a Resolver that fetches secret values
// from AWS Secrets Manager with periodic refresh.
package awssecretsmanager // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension/internal/awssecretsmanager"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.uber.org/zap"
)

const (
	DefaultRefreshInterval = 30 * time.Minute
	fetchTimeout           = 30 * time.Second
)

// SecretsManagerClient is the subset of the AWS Secrets Manager API used by Resolver.
type SecretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// Resolver fetches a secret from AWS Secrets Manager and periodically refreshes it.
// The caller provides an onFetch callback that receives the raw secret string
// and handles parsing and storage.
type Resolver struct {
	secretARN       string
	region          string
	refreshInterval time.Duration
	logger          *zap.Logger
	onFetch         func(string) error

	Client SecretsManagerClient
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewResolver creates a new AWS Secrets Manager resolver.
// The onFetch callback is called with the raw secret string on each successful fetch.
// If onFetch returns an error during Start, the error is propagated.
// If onFetch returns an error during refresh, the error is logged and the old value is retained.
func NewResolver(secretARN, region string, refreshInterval time.Duration, logger *zap.Logger, onFetch func(string) error) *Resolver {
	if refreshInterval <= 0 {
		refreshInterval = DefaultRefreshInterval
	}
	return &Resolver{
		secretARN:       secretARN,
		region:          region,
		refreshInterval: refreshInterval,
		logger:          logger,
		onFetch:         onFetch,
	}
}

func (r *Resolver) Start(ctx context.Context) error {
	if r.Client == nil {
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(r.region))
		if err != nil {
			return fmt.Errorf("load AWS config: %w", err)
		}
		r.Client = secretsmanager.NewFromConfig(cfg)
	}

	raw, err := r.fetch(ctx)
	if err != nil {
		return fmt.Errorf("initial secret fetch: %w", err)
	}
	if err := r.onFetch(raw); err != nil {
		return fmt.Errorf("initial secret processing: %w", err)
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	r.wg.Add(1)
	go r.refreshLoop(loopCtx)

	return nil
}

func (r *Resolver) Shutdown() error {
	if r.cancel != nil {
		r.cancel()
		r.wg.Wait()
	}
	return nil
}

func (r *Resolver) refreshLoop(ctx context.Context) {
	defer r.wg.Done()
	ticker := time.NewTicker(r.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			raw, err := r.fetch(ctx)
			if err != nil {
				r.logger.Error("failed to refresh secret", zap.String("secret_arn", r.secretARN), zap.Error(err))
				continue
			}
			if err := r.onFetch(raw); err != nil {
				r.logger.Error("failed to process refreshed secret", zap.String("secret_arn", r.secretARN), zap.Error(err))
			}
		}
	}
}

func (r *Resolver) fetch(ctx context.Context) (string, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	out, err := r.Client.GetSecretValue(fetchCtx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(r.secretARN),
	})
	if err != nil {
		return "", fmt.Errorf("get secret value: %w", err)
	}

	if out.SecretString == nil {
		return "", fmt.Errorf("secret %q has no string value (binary secrets are not supported)", r.secretARN)
	}

	return *out.SecretString, nil
}
