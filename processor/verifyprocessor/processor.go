// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
)

type verifyProcessor struct {
	nextConsumer consumer.Logs
}

func newProcessor(_ *Config, nextConsumer consumer.Logs, _ processor.Settings) *verifyProcessor {
	return &verifyProcessor{nextConsumer: nextConsumer}
}

func (*verifyProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*verifyProcessor) Shutdown(_ context.Context) error {
	return nil
}

func (*verifyProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (p *verifyProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return p.nextConsumer.ConsumeLogs(ctx, ld)
}
