// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awssecretsmanagerauthextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awssecretsmanagerauthextension"

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.uber.org/zap"
)

type secretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type poller struct {
	client          secretsManagerClient
	secretARN       string
	refreshInterval time.Duration
	logger          *zap.Logger
	cancel          context.CancelFunc
	lastVersionID   string
}

func newPoller(cfg *Config, logger *zap.Logger) (*poller, error) {
	awsCfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}
	return &poller{
		client:          secretsmanager.NewFromConfig(awsCfg),
		secretARN:       cfg.SecretARN,
		refreshInterval: cfg.RefreshInterval,
		logger:          logger,
	}, nil
}

// Start fetches the secret immediately and calls onChange, then polls on RefreshInterval.
// onChange is called only when the secret VersionId changes. Start is non-blocking.
func (p *poller) Start(ctx context.Context, onChange func(string)) error {
	value, versionID, err := p.fetch(ctx)
	if err != nil {
		return err
	}
	p.lastVersionID = versionID
	onChange(value)

	pollCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	go func() {
		ticker := time.NewTicker(p.refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-pollCtx.Done():
				return
			case <-ticker.C:
				v, vid, fetchErr := p.fetch(pollCtx)
				if fetchErr != nil {
					p.logger.Warn("failed to fetch secret from AWS Secrets Manager, keeping last known credentials",
						zap.String("secret_arn", p.secretARN),
						zap.Error(fetchErr))
					continue
				}
				if vid != p.lastVersionID {
					p.lastVersionID = vid
					onChange(v)
				}
			}
		}
	}()

	return nil
}

func (p *poller) Shutdown() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *poller) fetch(ctx context.Context) (value, versionID string, err error) {
	out, err := p.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &p.secretARN,
	})
	if err != nil {
		return "", "", err
	}
	if out.SecretString != nil {
		value = *out.SecretString
	}
	if out.VersionId != nil {
		versionID = *out.VersionId
	}
	return value, versionID, nil
}
