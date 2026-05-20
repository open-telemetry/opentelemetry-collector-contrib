// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/azuresecretmanagerauthextension/internal/azureprovider"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

const fetchTimeout = 30 * time.Second

type secretsClient interface {
	GetSecret(ctx context.Context, name, version string, options *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error)
}

type Config struct {
	KeyVaultURI     string
	SecretName      string
	RefreshInterval time.Duration
	Logger          *zap.Logger
	FetchFunc       func(ctx context.Context, secretValue string) error
}

type Provider struct {
	config *Config
	logger *zap.Logger
	Client secretsClient
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
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return fmt.Errorf("create Azure credential: %w", err)
		}
		client, err := azsecrets.NewClient(p.config.KeyVaultURI, cred, nil)
		if err != nil {
			return fmt.Errorf("create Azure Key Vault client: %w", err)
		}
		p.Client = client
	}

	if err := p.refresh(ctx); err != nil {
		return fmt.Errorf("initial credential fetch failed: %w", err)
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

	resp, err := p.Client.GetSecret(fetchCtx, p.config.SecretName, "", nil)
	if err != nil {
		return fmt.Errorf("failed to get secret value: %w", err)
	}
	if resp.Value == nil {
		return fmt.Errorf("secret %q has no value", p.config.SecretName)
	}

	return p.config.FetchFunc(ctx, *resp.Value)
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
