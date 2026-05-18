// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerauthextension/internal/awsprovider"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

const fetchTimeout = 30 * time.Second

type secretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput,
		optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type Config struct {
	SecretARN       string
	Region          string
	RefreshInterval time.Duration
	Logger          *zap.Logger
	FetchFunc       func(ctx context.Context, secretValue string) error
}

type Provider struct {
	config *Config
	logger *zap.Logger
	Client secretsManagerClient
	ticker *time.Ticker
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewProvider(cfg *Config) *Provider {
	return &Provider{
		config: cfg,
		logger: cfg.Logger,
	}
}

func (p *Provider) Start(ctx context.Context, _ component.Host) error {
	if p.Client == nil {
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.config.Region))
		if err != nil {
			return fmt.Errorf("load AWS config: %w", err)
		}
		p.Client = secretsmanager.NewFromConfig(cfg)
	}

	if err := p.refresh(ctx); err != nil {
		return fmt.Errorf("initial credential fetch: %w", err)
	}

	loopCtx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.ticker = time.NewTicker(p.config.RefreshInterval)

	p.wg.Add(1)
	go p.refreshLoop(loopCtx)

	return nil
}

func (p *Provider) Shutdown(_ context.Context) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.ticker != nil {
		p.ticker.Stop()
	}
	p.wg.Wait()
	return nil
}

func (p *Provider) refresh(ctx context.Context) error {
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	out, err := p.Client.GetSecretValue(fetchCtx, &secretsmanager.GetSecretValueInput{
		SecretId:     &p.config.SecretARN,
		VersionStage: aws.String("AWSCURRENT"),
	})
	if err != nil {
		return fmt.Errorf("get secret value: %w", err)
	}
	if out.SecretString == nil {
		return fmt.Errorf("secret %q has no string value", p.config.SecretARN)
	}

	return p.config.FetchFunc(ctx, *out.SecretString)
}

func (p *Provider) refreshLoop(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.ticker.C:
			if err := p.refresh(ctx); err != nil {
				p.logger.Error("failed to refresh credentials", zap.Error(err))
			}
		}
	}
}
