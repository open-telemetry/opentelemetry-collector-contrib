// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

type lookupProcessor struct {
	source lookupsource.Source
	logger *zap.Logger
}

func newLookupProcessor(source lookupsource.Source, logger *zap.Logger) *lookupProcessor {
	return &lookupProcessor{source: source, logger: logger}
}

func (p *lookupProcessor) Start(ctx context.Context, host component.Host) error {
	return p.source.Start(ctx, host)
}

func (p *lookupProcessor) Shutdown(ctx context.Context) error {
	return p.source.Shutdown(ctx)
}

func (*lookupProcessor) processLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	// Skeleton: passthrough for now
	return ld, nil
}
