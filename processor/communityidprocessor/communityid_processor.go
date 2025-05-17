// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package communityidprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/communityidprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type communityIdProcessor struct{}

func newCommunityIdProcessor() *communityIdProcessor {
	return &communityIdProcessor{}
}

func (g *communityIdProcessor) processMetrics(_ context.Context, ms pmetric.Metrics) (pmetric.Metrics, error) {
	return ms, nil
}

func (g *communityIdProcessor) processTraces(_ context.Context, ts ptrace.Traces) (ptrace.Traces, error) {
	return ts, nil
}

func (g *communityIdProcessor) processLogs(_ context.Context, ls plog.Logs) (plog.Logs, error) {
	return ls, nil
}
