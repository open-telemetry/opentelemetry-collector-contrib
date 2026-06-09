// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package awssecretsmanager provides a ValueResolver that fetches secret values
// from AWS Secrets Manager with periodic refresh.
package awssecretsmanager // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension/internal/awssecretsmanager"

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
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

// Resolver implements credentialsfile.ValueResolver by fetching
// a value from AWS Secrets Manager and periodically refreshing it.
type Resolver struct {
	secretARN       string
	region          string
	valueKey        string
	refreshInterval time.Duration
	logger          *zap.Logger
	onChange        func(string)

	value  atomic.Pointer[string]
	Client SecretsManagerClient
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewResolver creates a new AWS Secrets Manager resolver.
// If valueKey is non-empty, the secret is parsed as JSON and the specified key is extracted.
// Otherwise, the raw secret string is returned.
func NewResolver(secretARN, region, valueKey string, refreshInterval time.Duration, logger *zap.Logger, onChange func(string)) *Resolver {
	if refreshInterval <= 0 {
		refreshInterval = DefaultRefreshInterval
	}
	return &Resolver{
		secretARN:       secretARN,
		region:          region,
		valueKey:        valueKey,
		refreshInterval: refreshInterval,
		logger:          logger,
		onChange:        onChange,
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

	val, err := r.fetch(ctx)
	if err != nil {
		return fmt.Errorf("initial secret fetch: %w", err)
	}
	r.value.Store(&val)

	loopCtx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	r.wg.Add(1)
	go r.refreshLoop(loopCtx)

	return nil
}

func (r *Resolver) Value() string {
	if v := r.value.Load(); v != nil {
		return *v
	}
	return ""
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
			val, err := r.fetch(ctx)
			if err != nil {
				r.logger.Error("failed to refresh secret", zap.String("secret_arn", r.secretARN), zap.Error(err))
				continue
			}
			old := r.value.Load()
			if old == nil || *old != val {
				r.value.Store(&val)
				if r.onChange != nil {
					r.onChange(val)
				}
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

	raw := *out.SecretString

	if r.valueKey == "" {
		return raw, nil
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", fmt.Errorf("parse secret as JSON: %w", err)
	}

	val, ok := parsed[r.valueKey]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret JSON", r.valueKey)
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("key %q in secret is not a string", r.valueKey)
	}

	return str, nil
}
