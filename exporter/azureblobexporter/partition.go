// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

// signalOps adapts a pdata payload type (plog.Logs, pmetric.Metrics or
// ptrace.Traces) so that partitioning and partial-failure handling can be
// shared across signals.
type signalOps[T any] struct {
	signal     pipeline.Signal
	newPayload func() T
	// resourceCount returns the number of resource entries in the payload.
	resourceCount func(T) int
	// view returns resource entry i of the payload as a blob name template's
	// root object, without copying it.
	view func(data T, i int) any
	// copyResource appends a copy of resource entry i of src to dst.
	copyResource func(src T, i int, dst T)
	// moveResources moves every resource entry of src to the end of dst.
	moveResources func(src, dst T)
	// defaultFormat returns the configured (non-rendered) blob name format.
	defaultFormat func(BlobNameFormat) string
	// partialError wraps err so exporterhelper retries only the given data.
	partialError func(err error, data T) error
}

var logsOps = signalOps[plog.Logs]{
	signal:        pipeline.SignalLogs,
	newPayload:    plog.NewLogs,
	resourceCount: func(ld plog.Logs) int { return ld.ResourceLogs().Len() },
	view:          func(ld plog.Logs, i int) any { return logsView{ld.ResourceLogs().At(i)} },
	copyResource: func(src plog.Logs, i int, dst plog.Logs) {
		src.ResourceLogs().At(i).CopyTo(dst.ResourceLogs().AppendEmpty())
	},
	moveResources: func(src, dst plog.Logs) { src.ResourceLogs().MoveAndAppendTo(dst.ResourceLogs()) },
	defaultFormat: func(f BlobNameFormat) string { return f.LogsFormat },
	partialError:  consumererror.NewLogs,
}

var metricsOps = signalOps[pmetric.Metrics]{
	signal:        pipeline.SignalMetrics,
	newPayload:    pmetric.NewMetrics,
	resourceCount: func(md pmetric.Metrics) int { return md.ResourceMetrics().Len() },
	view:          func(md pmetric.Metrics, i int) any { return metricsView{md.ResourceMetrics().At(i)} },
	copyResource: func(src pmetric.Metrics, i int, dst pmetric.Metrics) {
		src.ResourceMetrics().At(i).CopyTo(dst.ResourceMetrics().AppendEmpty())
	},
	moveResources: func(src, dst pmetric.Metrics) { src.ResourceMetrics().MoveAndAppendTo(dst.ResourceMetrics()) },
	defaultFormat: func(f BlobNameFormat) string { return f.MetricsFormat },
	partialError:  consumererror.NewMetrics,
}

var tracesOps = signalOps[ptrace.Traces]{
	signal:        pipeline.SignalTraces,
	newPayload:    ptrace.NewTraces,
	resourceCount: func(td ptrace.Traces) int { return td.ResourceSpans().Len() },
	view:          func(td ptrace.Traces, i int) any { return tracesView{td.ResourceSpans().At(i)} },
	copyResource: func(src ptrace.Traces, i int, dst ptrace.Traces) {
		src.ResourceSpans().At(i).CopyTo(dst.ResourceSpans().AppendEmpty())
	},
	moveResources: func(src, dst ptrace.Traces) { src.ResourceSpans().MoveAndAppendTo(dst.ResourceSpans()) },
	defaultFormat: func(f BlobNameFormat) string { return f.TracesFormat },
	partialError:  consumererror.NewTraces,
}

type blobGroup[T any] struct {
	data T
	name resolvedBlobName
}

// export uploads the payload. With template_scope resource, it is partitioned
// by blob name first; otherwise it is uploaded to a single blob.
func export[T any](
	ctx context.Context,
	e *azureBlobExporter,
	ops signalOps[T],
	data T,
	marshal func(T) ([]byte, error),
) error {
	format := e.config.BlobNameFormat
	if format.TemplateEnabled && format.TemplateScope == TemplateScopeResource && ops.resourceCount(data) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		return uploadGroups(ctx, e, ops, partitionByBlobName(e, ops, data, e.now()), marshal)
	}

	payload, err := marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", ops.signal.String(), err)
	}
	blobName, err := e.generateBlobNameWithCompression(ops.signal, data)
	if err != nil {
		return fmt.Errorf("failed to generate blobname: %w", err)
	}
	return e.consumeData(ctx, blobName, payload, ops.signal)
}

// partitionByBlobName splits the payload into groups of resource entries whose
// blob name templates resolve to the same blob name at now, so that each group
// is uploaded to the blob it is addressed to.
//
// Templates are evaluated against a view of each resource entry, so resource
// entries are only copied when they resolve to more than one blob name, and
// then only once each. The original payload is returned as a single group when
// every resource entry resolves to the same blob name, or when rendering fails
// for any resource entry. In the failure case the default blob name format is
// used, matching generateBlobNameWithCompression.
func partitionByBlobName[T any](e *azureBlobExporter, ops signalOps[T], data T, now time.Time) []blobGroup[T] {
	count := ops.resourceCount(data)
	groupOf := make([]int, count)
	indices := make(map[resolvedBlobName]int)
	var names []resolvedBlobName
	for i := range count {
		rendered, err := e.renderBlobNameTemplate(ops.signal, ops.view(data, i))
		if err != nil {
			e.logger.Warn("Failed to execute blob name template, using default blob name format", zap.Error(err))
			name := e.resolveBlobName(now, ops.defaultFormat(e.config.BlobNameFormat))
			return []blobGroup[T]{{data: data, name: name}}
		}

		name := e.resolveBlobName(now, rendered)
		index, ok := indices[name]
		if !ok {
			index = len(names)
			indices[name] = index
			names = append(names, name)
		}
		groupOf[i] = index
	}

	if len(names) == 1 {
		return []blobGroup[T]{{data: data, name: names[0]}}
	}
	groups := make([]blobGroup[T], len(names))
	for index, name := range names {
		groups[index] = blobGroup[T]{data: ops.newPayload(), name: name}
	}
	for i, index := range groupOf {
		ops.copyResource(data, i, groups[index].data)
	}
	return groups
}

// uploadGroups marshals and uploads each partitioned group. A single group is
// uploaded and any error is returned as-is, preserving the non-partitioned
// behavior. Groups are uploaded sequentially; exporterhelper controls
// concurrency between export requests. If some groups fail, the error carries
// only failed and unstarted data for exporterhelper to retry.
func uploadGroups[T any](
	ctx context.Context,
	e *azureBlobExporter,
	ops signalOps[T],
	groups []blobGroup[T],
	marshal func(T) ([]byte, error),
) error {
	upload := func(group blobGroup[T]) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		data, err := marshal(group.data)
		if err != nil {
			return fmt.Errorf("failed to marshal %s: %w", ops.signal.String(), err)
		}
		return e.consumeData(ctx, e.finalizeBlobName(group.name), data, ops.signal)
	}
	if len(groups) == 1 {
		return upload(groups[0])
	}

	var failed []T
	var errs []error
	for i, group := range groups {
		if err := ctx.Err(); err != nil {
			for _, unstarted := range groups[i:] {
				failed = append(failed, unstarted.data)
			}
			errs = append(errs, err)
			break
		}
		if err := upload(group); err != nil {
			failed = append(failed, group.data)
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	combined := ops.newPayload()
	for _, f := range failed {
		ops.moveResources(f, combined)
	}
	return ops.partialError(errors.Join(errs...), combined)
}
