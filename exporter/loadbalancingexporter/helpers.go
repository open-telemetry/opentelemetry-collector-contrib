// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
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

// joinPartialFailure builds the error a ConsumeTraces/ConsumeLogs/ConsumeMetrics call
// returns for a batch with per-endpoint failures, given every failed shard's data already
// embedded by the caller (both retryable and permanent). If any endpoint failed retryably,
// only the retryable errors go into the returned error's unwrap tree, so
// consumererror.IsPermanent stays false and a parent retry re-attempts the whole embedded
// set, including the permanently-failed shards: dropping their data instead would let a
// later successful retry of the retryable remainder report the whole original request as
// sent, silently hiding the permanent loss from ObsReportSender. Re-attempting a permanent
// shard fails it again with the same permanent error, so once only permanent failures
// remain, the all-permanent branch below fires and the retry sender stops - no delivered
// duplicates result, because every re-attempted permanent send fails. If every endpoint
// failed permanently, their errors are joined directly, so IsPermanent is true immediately
// and the parent retry sender drops the batch instead of looping on an error it cannot
// recover from.
func joinPartialFailure(retryableErrs, permanentErrs []error) error {
	switch {
	case len(retryableErrs) == 0:
		return errors.Join(permanentErrs...)
	case len(permanentErrs) == 0:
		return errors.Join(retryableErrs...)
	default:
		return fmt.Errorf("%w (%d endpoint(s) also failed permanently and are re-attempted with the retryable subset: %s)",
			errors.Join(retryableErrs...), len(permanentErrs), errors.Join(permanentErrs...))
	}
}
