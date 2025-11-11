// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotreloadprocessor

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	otelcol "go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/pdata/plog"
	otelprocessor "go.opentelemetry.io/collector/processor"
)

type HotReloadLogsProcessor struct {
	*HotReloadProcessor[consumer.Logs, otelprocessor.Logs]
}

func newHotReloadLogsProcessor(
	context context.Context,
	set otelprocessor.Settings,
	cfg *Config,
	nextConsumer consumer.Logs,
) (*HotReloadLogsProcessor, error) {
	hp, err := newHotReloadProcessor(
		context,
		set,
		cfg,
		nextConsumer,
		loaderLogs{},
	)
	if err != nil {
		return nil, fmt.Errorf("error creating hotreload processor: %w", err)
	}
	return &HotReloadLogsProcessor{hp}, nil
}

func (hp *HotReloadLogsProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return hp.consume(func() error {
		fsp := hp.firstSubprocessor.Load()
		if fsp == nil {
			return nil
		}
		return (*fsp).ConsumeLogs(ctx, ld)
	})
}

type loaderLogs struct{}

func (l loaderLogs) load(
	ctx context.Context,
	config otelcol.Config,
	set otelprocessor.Settings,
	host component.Host,
	nextConsumer consumer.Logs,
) ([]otelprocessor.Logs, error) {
	proc, err := loadLogsSubprocessors(ctx, config, set, host, nextConsumer)
	if err != nil {
		return nil, err
	}

	if len(proc) == 0 {
		// Return a passthrough processor if none are loaded
		return []otelprocessor.Logs{&passthroughLogsProcessor{next: nextConsumer}}, nil
	}

	proc2 := make([]otelprocessor.Logs, len(proc))
	for i, p := range proc {
		proc2[i] = p.(otelprocessor.Logs)
	}
	return proc2, nil
}

// passthroughLogsProcessor is a fallback processor that just forwards logs to nextConsumer.
type passthroughLogsProcessor struct {
	next consumer.Logs
}

func (p *passthroughLogsProcessor) Start(ctx context.Context, host component.Host) error {
	// No-op
	return nil
}

func (p *passthroughLogsProcessor) Shutdown(ctx context.Context) error {
	// No-op
	return nil
}

func (p *passthroughLogsProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return p.next.ConsumeLogs(ctx, ld)
}

func (p *passthroughLogsProcessor) Capabilities() consumer.Capabilities {
	return processorCapabilities
}
