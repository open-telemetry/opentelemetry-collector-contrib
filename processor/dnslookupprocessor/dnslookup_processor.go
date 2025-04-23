// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type dnsLookupProcessor struct{}

func newDNSLookupProcessor() *dnsLookupProcessor {
	return &dnsLookupProcessor{}
}

func (g *dnsLookupProcessor) processMetrics(_ context.Context, ms pmetric.Metrics) (pmetric.Metrics, error) {
	return ms, nil
}

func (g *dnsLookupProcessor) processTraces(_ context.Context, ts ptrace.Traces) (ptrace.Traces, error) {
	return ts, nil
}

func (g *dnsLookupProcessor) processLogs(_ context.Context, ls plog.Logs) (plog.Logs, error) {
	return ls, nil
}
