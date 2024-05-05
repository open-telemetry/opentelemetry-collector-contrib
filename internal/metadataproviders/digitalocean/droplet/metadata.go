// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package droplet // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/digitalocean/droplet"

import (
	"context"

	droplet "github.com/digitalocean/go-metadata"
)

type Provider interface {
	Get(ctx context.Context) (*droplet.Metadata, error)
}

type dropletProviderImpl struct {
	client *droplet.Client
}

// NewProvider creates a new metadata provider
func NewProvider(client *droplet.Client) Provider {
	return &dropletProviderImpl{
		client: client,
	}
}

func (d *dropletProviderImpl) Get(ctx context.Context) (*droplet.Metadata, error) {
	return d.client.Metadata()
}
