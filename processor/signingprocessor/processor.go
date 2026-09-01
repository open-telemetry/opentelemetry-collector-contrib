// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
)

type signingProcessor struct {
	nextConsumer consumer.Logs
}

func newProcessor(_ *Config, nextConsumer consumer.Logs, _ processor.Settings) *signingProcessor {
	return &signingProcessor{nextConsumer: nextConsumer}
}

func (*signingProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*signingProcessor) Shutdown(_ context.Context) error {
	return nil
}

func (*signingProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *signingProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return p.nextConsumer.ConsumeLogs(ctx, ld)
}
