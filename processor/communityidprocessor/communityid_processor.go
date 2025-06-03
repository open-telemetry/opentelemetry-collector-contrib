// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package communityidprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/communityidprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type communityIDProcessor struct{}

func newCommunityIDProcessor() *communityIDProcessor {
	return &communityIDProcessor{}
}

func (g *communityIDProcessor) processMetrics(_ context.Context, ms pmetric.Metrics) (pmetric.Metrics, error) {
	return ms, nil
}

func (g *communityIDProcessor) processTraces(_ context.Context, ts ptrace.Traces) (ptrace.Traces, error) {
	return ts, nil
}

func (g *communityIDProcessor) processLogs(_ context.Context, ls plog.Logs) (plog.Logs, error) {
	return ls, nil
}
