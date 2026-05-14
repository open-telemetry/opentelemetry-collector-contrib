// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.uber.org/zap"
)

type secretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// secretCredentials holds a consistent username/password pair fetched from a single secret.
// Stored as a single atomic pointer to guarantee HTTP clients always read a matched pair.
type secretCredentials struct {
	username string
	password string
}

// awsSecretsManagerResolver fetches a JSON secret from AWS Secrets Manager and polls
// for changes at a configurable interval, updating credentials in place without restarting
// the collector.
type awsSecretsManagerResolver struct {
	cfg        *AWSSecretsManagerSettings
	client     secretsManagerClient
	creds      atomic.Pointer[secretCredentials]
	onChange   func()
	shutdownCh chan struct{}
	doneCh     chan struct{}
	logger     *zap.Logger
}

func newAWSSecretsManagerResolver(cfg *AWSSecretsManagerSettings, logger *zap.Logger, onChange func()) *awsSecretsManagerResolver {
	return &awsSecretsManagerResolver{
		cfg:      cfg,
		logger:   logger,
		onChange: onChange,
	}
}

func (r *awsSecretsManagerResolver) start(ctx context.Context) error {
	opts := []func(*awsconfig.LoadOptions) error{}
	if r.cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(r.cfg.Region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	r.client = secretsmanager.NewFromConfig(cfg)
	return r.startWithClient(ctx)
}

func (r *awsSecretsManagerResolver) startWithClient(ctx context.Context) error {
	if r.shutdownCh != nil {
		return errors.New("already started")
	}
	if err := r.fetch(ctx); err != nil {
		return fmt.Errorf("initial fetch from AWS Secrets Manager failed: %w", err)
	}

	r.shutdownCh = make(chan struct{})
	r.doneCh = make(chan struct{})
	go r.poll(r.cfg.refreshInterval())
	return nil
}

func (r *awsSecretsManagerResolver) shutdown() {
	if r.shutdownCh != nil {
		close(r.shutdownCh)
		<-r.doneCh
		r.shutdownCh = nil
	}
}

func (r *awsSecretsManagerResolver) Username() string {
	if c := r.creds.Load(); c != nil {
		return c.username
	}
	return ""
}

func (r *awsSecretsManagerResolver) Password() string {
	if c := r.creds.Load(); c != nil {
		return c.password
	}
	return ""
}

func (r *awsSecretsManagerResolver) poll(interval time.Duration) {
	defer close(r.doneCh)

	// cancelCtx lets in-flight AWS calls be interrupted when shutdown is requested,
	// avoiding a long block waiting for the SDK timeout.
	// Capture the channel value (not the field) to avoid a race with shutdown()
	// zeroing r.shutdownCh after doneCh is closed.
	shutdownCh := r.shutdownCh
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-shutdownCh
		cancel()
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-r.shutdownCh:
			return
		case <-ticker.C:
			if err := r.fetch(ctx); err != nil {
				r.logger.Warn("failed to refresh credentials from AWS Secrets Manager, keeping last known values",
					zap.String("secret_arn", r.cfg.SecretARN),
					zap.Error(err))
			}
		}
	}
}

func (r *awsSecretsManagerResolver) fetch(ctx context.Context) error {
	resp, err := r.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(r.cfg.SecretARN),
	})
	if err != nil {
		return fmt.Errorf("GetSecretValue: %w", err)
	}
	if resp.SecretString == nil {
		return fmt.Errorf("secret %q has no string value", r.cfg.SecretARN)
	}

	var fields map[string]string
	if err := json.Unmarshal([]byte(*resp.SecretString), &fields); err != nil {
		return fmt.Errorf("unmarshal secret JSON: %w", err)
	}

	usernameKey := r.cfg.usernameKey()
	passwordKey := r.cfg.passwordKey()

	newUsername, ok := fields[usernameKey]
	if !ok {
		return fmt.Errorf("key %q not found in secret %q", usernameKey, r.cfg.SecretARN)
	}
	newPassword, ok := fields[passwordKey]
	if !ok {
		return fmt.Errorf("key %q not found in secret %q", passwordKey, r.cfg.SecretARN)
	}

	// Store both credentials atomically so HTTP clients always read a matched pair.
	old := r.creds.Load()
	if old != nil && old.username == newUsername && old.password == newPassword {
		return nil
	}
	r.creds.Store(&secretCredentials{username: newUsername, password: newPassword})
	if r.onChange != nil {
		r.onChange()
	}
	return nil
}
