// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

type credentials struct {
	username string
	password string
}

// ClientResolver fetches a JSON secret from AWS Secrets Manager and extracts
// both username and password in a single API call.
type ClientResolver struct {
	secretARN       string
	region          string
	usernameKey     string
	passwordKey     string
	refreshInterval time.Duration
	logger          *zap.Logger

	creds  atomic.Pointer[credentials]
	Client SecretsManagerClient
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewClientResolver(secretARN, region, usernameKey, passwordKey string, refreshInterval time.Duration, logger *zap.Logger) *ClientResolver {
	if refreshInterval <= 0 {
		refreshInterval = DefaultRefreshInterval
	}
	return &ClientResolver{
		secretARN:       secretARN,
		region:          region,
		usernameKey:     usernameKey,
		passwordKey:     passwordKey,
		refreshInterval: refreshInterval,
		logger:          logger,
	}
}

func (r *ClientResolver) Start(ctx context.Context) error {
	if r.Client == nil {
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(r.region))
		if err != nil {
			return fmt.Errorf("load AWS config: %w", err)
		}
		r.Client = secretsmanager.NewFromConfig(cfg)
	}

	if err := r.refresh(ctx); err != nil {
		return fmt.Errorf("initial secret fetch: %w", err)
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	r.wg.Add(1)
	go r.refreshLoop(loopCtx)

	return nil
}

func (r *ClientResolver) Username() string {
	if c := r.creds.Load(); c != nil {
		return c.username
	}
	return ""
}

func (r *ClientResolver) Password() string {
	if c := r.creds.Load(); c != nil {
		return c.password
	}
	return ""
}

func (r *ClientResolver) Shutdown() error {
	if r.cancel != nil {
		r.cancel()
		r.wg.Wait()
	}
	return nil
}

func (r *ClientResolver) refreshLoop(ctx context.Context) {
	defer r.wg.Done()
	ticker := time.NewTicker(r.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.refresh(ctx); err != nil {
				r.logger.Error("failed to refresh secret", zap.String("secret_arn", r.secretARN), zap.Error(err))
			}
		}
	}
}

func (r *ClientResolver) refresh(ctx context.Context) error {
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	out, err := r.Client.GetSecretValue(fetchCtx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(r.secretARN),
	})
	if err != nil {
		return fmt.Errorf("get secret value: %w", err)
	}

	if out.SecretString == nil {
		return fmt.Errorf("secret %q has no string value (binary secrets are not supported)", r.secretARN)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(*out.SecretString), &parsed); err != nil {
		return fmt.Errorf("parse secret as JSON: %w", err)
	}

	usernameVal, ok := parsed[r.usernameKey]
	if !ok {
		return fmt.Errorf("key %q not found in secret JSON", r.usernameKey)
	}
	username, ok := usernameVal.(string)
	if !ok {
		return fmt.Errorf("key %q in secret is not a string", r.usernameKey)
	}

	passwordVal, ok := parsed[r.passwordKey]
	if !ok {
		return fmt.Errorf("key %q not found in secret JSON", r.passwordKey)
	}
	password, ok := passwordVal.(string)
	if !ok {
		return fmt.Errorf("key %q in secret is not a string", r.passwordKey)
	}

	newCreds := &credentials{username: username, password: password}
	r.creds.CompareAndSwap(r.creds.Load(), newCreds)
	return nil
}
