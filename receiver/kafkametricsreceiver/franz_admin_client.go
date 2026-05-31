// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"sync"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

// franzAdminProvider lazily creates a single franz-go admin client shared by all
// of the receiver's scrapers. The client is created on first use and reference
// counted so it is closed only once every scraper has shut down.
type franzAdminProvider struct {
	clientConfig configkafka.ClientConfig
	logger       *zap.Logger

	mu       sync.Mutex
	adm      *kadm.Client
	cl       *kgo.Client
	refCount int
}

func newFranzAdminProvider(clientConfig configkafka.ClientConfig, logger *zap.Logger) *franzAdminProvider {
	return &franzAdminProvider{
		clientConfig: clientConfig,
		logger:       logger,
	}
}

// retain registers a user of the shared client; balance with one release.
func (p *franzAdminProvider) retain() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.refCount++
}

// admin returns the shared kadm client, creating it lazily on first use.
func (p *franzAdminProvider) admin(ctx context.Context, host component.Host) (*kadm.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.adm != nil {
		return p.adm, nil
	}
	adm, cl, err := kafka.NewFranzClusterAdminClient(ctx, host, p.clientConfig, p.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create franz-go admin client: %w", err)
	}
	p.adm = adm
	p.cl = cl
	return p.adm, nil
}

// release drops a reference, closing the client when the last one is released.
func (p *franzAdminProvider) release() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.refCount > 0 {
		p.refCount--
	}
	if p.refCount > 0 {
		return
	}
	p.closeLocked()
}

func (p *franzAdminProvider) closeLocked() {
	if p.adm != nil {
		// kadm.Client.Close also closes the underlying *kgo.Client.
		p.adm.Close()
		p.adm = nil
		p.cl = nil
	}
}
