// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipfixlookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/ipfixlookupprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// schema for processor
type processorImp struct {
	config         Config
	tracesConsumer consumer.Traces
	logger         *zap.Logger
}

func newProcessor(logger *zap.Logger, config component.Config) *processorImp {
	logger.Info("Building ipfixlookupprocessor processor")
	cfg := config.(*Config)

	return &processorImp{
		config: *cfg,
		logger: logger,
	}
}

func (p *processorImp) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *processorImp) Start(_ context.Context, _ component.Host) error {
	p.logger.Info("Starting ipfixlookupprocessor processor")
	return nil
}

func (p *processorImp) Shutdown(context.Context) error {
	p.logger.Info("Shutting down ipfixlookupprocessor processor")
	return nil
}

func (p *processorImp) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return p.tracesConsumer.ConsumeTraces(ctx, td)
}
