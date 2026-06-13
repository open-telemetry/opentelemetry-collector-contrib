// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcpprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/gcpsecretsmanagerauthextension/internal/gcpprovider"

import (
	"context"
	"fmt"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

const fetchTimeout = 30 * time.Second

type secretsClient interface {
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest,
		opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
	Close() error
}

type Config struct {
	Project         string
	SecretName      string
	RefreshInterval time.Duration
	Logger          *zap.Logger
	FetchFunc       func(ctx context.Context, secretValue []byte) error
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
		client, err := secretmanager.NewRESTClient(ctx)
		if err != nil {
			return fmt.Errorf("create GCP Secret Manager client: %w", err)
		}
		p.Client = client
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
	if p.Client != nil {
		return p.Client.Close()
	}
	return nil
}

func (p *Provider) refresh(ctx context.Context) error {
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	secretPath := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", p.config.Project, p.config.SecretName)

	resp, err := p.Client.AccessSecretVersion(fetchCtx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretPath,
	})
	if err != nil {
		return fmt.Errorf("access secret version: %w", err)
	}
	if resp.Payload == nil || resp.Payload.Data == nil {
		return fmt.Errorf("secret %q has no payload data", secretPath)
	}

	return p.config.FetchFunc(ctx, resp.Payload.Data)
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
