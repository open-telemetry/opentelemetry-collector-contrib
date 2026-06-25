// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerprovider"

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/secretprovider"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

const fetchTimeout = 30 * time.Second

type secretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput,
		optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

var (
	_ extension.Extension        = (*awsSecretProvider)(nil)
	_ secretprovider.SecretProvider = (*awsSecretProvider)(nil)
)

type awsSecretProvider struct {
	cfg      *Config
	logger   *zap.Logger
	client   secretsManagerClient
	secret   atomic.Pointer[string]
	onChange func(string)
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func newAWSSecretProvider(cfg *Config, logger *zap.Logger) *awsSecretProvider {
	return &awsSecretProvider{
		cfg:    cfg,
		logger: logger,
	}
}

func (p *awsSecretProvider) Start(ctx context.Context, _ component.Host) error {
	if p.client == nil {
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.cfg.Region))
		if err != nil {
			return fmt.Errorf("load AWS config: %w", err)
		}
		p.client = secretsmanager.NewFromConfig(cfg)
	}

	raw, err := p.fetchSecret(ctx)
	if err != nil {
		return fmt.Errorf("initial secret fetch: %w", err)
	}
	p.secret.Store(&raw)

	if p.cfg.RefreshInterval > 0 {
		rctx, cancel := context.WithCancel(context.Background())
		p.cancel = cancel
		p.wg.Add(1)
		go p.refreshLoop(rctx)
	}

	return nil
}

func (p *awsSecretProvider) Shutdown(_ context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	return nil
}

func (p *awsSecretProvider) GetSecret(_ context.Context) (string, error) {
	v := p.secret.Load()
	if v == nil {
		return "", fmt.Errorf("secret not yet loaded")
	}
	return *v, nil
}

func (p *awsSecretProvider) OnChange(fn func(string)) {
	p.onChange = fn
}

func (p *awsSecretProvider) refreshLoop(ctx context.Context) {
	defer p.wg.Done()
	ticker := time.NewTicker(p.cfg.RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			raw, err := p.fetchSecret(ctx)
			if err != nil {
				p.logger.Error("failed to refresh secret", zap.Error(err))
				continue
			}
			old := p.secret.Load()
			if old != nil && *old == raw {
				continue
			}
			p.secret.Store(&raw)
			p.logger.Info("secret value rotated")
			if p.onChange != nil {
				p.onChange(raw)
			}
		}
	}
}

func (p *awsSecretProvider) fetchSecret(ctx context.Context) (string, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	out, err := p.client.GetSecretValue(fetchCtx, &secretsmanager.GetSecretValueInput{
		SecretId:     &p.cfg.SecretARN,
		VersionStage: aws.String("AWSCURRENT"),
	})
	if err != nil {
		return "", fmt.Errorf("get secret value: %w", err)
	}
	if out.SecretString == nil {
		return "", fmt.Errorf("secret %q has no string value", p.cfg.SecretARN)
	}
	return *out.SecretString, nil
}
