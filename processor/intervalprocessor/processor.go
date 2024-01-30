// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package intervalprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

var _ processor.Metrics = (*Processor)(nil)

type Processor struct {
	ctx    context.Context
	cancel context.CancelFunc
	log    *zap.Logger

	maxStaleness time.Duration

	nextConsumer consumer.Metrics
}

func newProcessor(config *Config, log *zap.Logger, nextConsumer consumer.Metrics) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Processor{
		ctx:          ctx,
		cancel:       cancel,
		log:          log,
		maxStaleness: config.MaxStaleness,
		nextConsumer: nextConsumer,
	}
}

func (p *Processor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (p *Processor) Shutdown(_ context.Context) error {
	p.cancel()
	return nil
}

func (p *Processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *Processor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return p.nextConsumer.ConsumeMetrics(ctx, md)
}
