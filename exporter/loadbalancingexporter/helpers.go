// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
)

// mergeTraces concatenates two ptrace.Traces into a single ptrace.Traces.
func mergeTraces(t1, t2 ptrace.Traces) ptrace.Traces {
	t2.ResourceSpans().MoveAndAppendTo(t1.ResourceSpans())
	return t1
}

// mergeLogs concatenates two plog.Logs into a single plog.Logs.
func mergeLogs(l1, l2 plog.Logs) plog.Logs {
	l2.ResourceLogs().MoveAndAppendTo(l1.ResourceLogs())
	return l1
}

// copyTracesInto deep-copies src's resource spans onto the end of dst, leaving src
// untouched. Used to accumulate failed-endpoint data for consumererror.NewTraces without
// aliasing pdata that a child exporter may still hold a reference to.
func copyTracesInto(dst, src ptrace.Traces) {
	rss := src.ResourceSpans()
	for i := range rss.Len() {
		rss.At(i).CopyTo(dst.ResourceSpans().AppendEmpty())
	}
}

// copyLogsInto deep-copies src's resource logs onto the end of dst, leaving src
// untouched. Used to accumulate failed-endpoint data for consumererror.NewLogs without
// aliasing pdata that a child exporter may still hold a reference to.
func copyLogsInto(dst, src plog.Logs) {
	rls := src.ResourceLogs()
	for i := range rls.Len() {
		rls.At(i).CopyTo(dst.ResourceLogs().AppendEmpty())
	}
}

// copyMetricsInto deep-copies src's resource metrics onto the end of dst, leaving src
// untouched. Used instead of metrics.Merge to accumulate failed-endpoint data: Merge's
// identity model ignores ResourceMetrics/ScopeMetrics SchemaUrl, so distinct-schema shards
// with otherwise-identical resources/scopes would silently collapse into one entry.
func copyMetricsInto(dst, src pmetric.Metrics) {
	rms := src.ResourceMetrics()
	for i := range rms.Len() {
		rms.At(i).CopyTo(dst.ResourceMetrics().AppendEmpty())
	}
}

type backendFailures struct {
	err          error
	hasPermanent bool
}

// add records err and reports whether its data can be retried.
func (f *backendFailures) add(err error) (retryable bool) {
	f.err = multierr.Append(f.err, err)
	if consumererror.IsPermanent(err) {
		f.hasPermanent = true
		return false
	}
	return true
}
