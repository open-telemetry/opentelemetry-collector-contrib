// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package certificatelogverifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/certificatelogverifyprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
)

type certificateLogVerifyProcessor struct {
	nextConsumer consumer.Logs
}

func newProcessor(_ *Config, nextConsumer consumer.Logs, _ processor.Settings) *certificateLogVerifyProcessor {
	return &certificateLogVerifyProcessor{nextConsumer: nextConsumer}
}

func (*certificateLogVerifyProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*certificateLogVerifyProcessor) Shutdown(_ context.Context) error {
	return nil
}

func (*certificateLogVerifyProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *certificateLogVerifyProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return p.nextConsumer.ConsumeLogs(ctx, ld)
}
