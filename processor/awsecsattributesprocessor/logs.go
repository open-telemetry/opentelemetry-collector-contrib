// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logsProcessor struct {
	*ecsCore
	next consumer.Logs
}

func newLogsProcessor(logger *zap.Logger, cfg *Config, next consumer.Logs, endpoints endpointsFn) *logsProcessor {
	return &logsProcessor{ecsCore: newCore(logger, cfg, endpoints), next: next}
}

func (p *logsProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	rls := ld.ResourceLogs()
	for i := range rls.Len() {
		p.enrichResource(ctx, rls.At(i).Resource())
	}
	return p.next.ConsumeLogs(ctx, ld)
}
