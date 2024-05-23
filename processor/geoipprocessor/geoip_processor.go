// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type geoIPProcessor struct{}

func newGeoIPProcessor() *geoIPProcessor {
	return &geoIPProcessor{}
}

func (g *geoIPProcessor) processMetrics(ctx context.Context, ms pmetric.Metrics) (pmetric.Metrics, error) {
	return ms, nil
}

func (g *geoIPProcessor) processTraces(ctx context.Context, ts ptrace.Traces) (ptrace.Traces, error) {
	return ts, nil
}

func (g *geoIPProcessor) processLogs(ctx context.Context, ls plog.Logs) (plog.Logs, error) {
	return ls, nil
}
