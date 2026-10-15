// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"context"
	"errors"
	"fmt"

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
	copyResource: func(src ptrace.Traces, i int, dst ptrace.Traces) {
		src.ResourceSpans().At(i).CopyTo(dst.ResourceSpans().AppendEmpty())
	},
	moveResources: func(src, dst ptrace.Traces) { src.ResourceSpans().MoveAndAppendTo(dst.ResourceSpans()) },
	defaultFormat: func(f BlobNameFormat) string { return f.TracesFormat },
	partialError:  consumererror.NewTraces,
}

type blobGroup[T any] struct {
	data T
	// A non-nil format is already rendered or explicitly selected for fallback.
	// Never re-render it against the combined group.
	nameFormat *string
}

// exportPartitioned partitions the payload by rendered blob name and uploads
// each group.
func exportPartitioned[T any](
	ctx context.Context,
	e *azureBlobExporter,
	ops signalOps[T],
	data T,
	marshal func(T) ([]byte, error),
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return uploadGroups(ctx, e, ops, partitionByBlobName(e, ops, data), marshal)
}

// partitionByBlobName splits the payload into groups of resource entries whose
// blob name templates render to the same value, so that each group is uploaded
// to the blob it is addressed to. Without this, the blob name would be rendered
// once from the first resource entry and data belonging to other resource
// entries would be written to that same blob.
//
// The original payload is returned as a single group when the template is
// disabled, when every resource entry renders to the same blob name, or when
// rendering fails for any resource entry. In the failure case the upload path
// falls back to the default blob name format, matching generateBlobName.
func partitionByBlobName[T any](e *azureBlobExporter, ops signalOps[T], data T) []blobGroup[T] {
	count := ops.resourceCount(data)
	if !e.config.BlobNameFormat.TemplateEnabled || count <= 1 {
		return []blobGroup[T]{{data: data}}
	}

	indices := make(map[string]int)
	var groups []blobGroup[T]
	for i := range count {
		single := ops.newPayload()
		ops.copyResource(data, i, single)

		name, err := e.renderBlobNameTemplate(ops.signal, single)
		if err != nil {
			e.logger.Warn("Failed to execute blob name template, using default blob name format", zap.Error(err))
			format := ops.defaultFormat(e.config.BlobNameFormat)
			return []blobGroup[T]{{data: data, nameFormat: &format}}
		}

		index, ok := indices[name]
		if !ok {
			indices[name] = len(groups)
			groups = append(groups, blobGroup[T]{data: single, nameFormat: &name})
			continue
		}
		ops.moveResources(single, groups[index].data)
	}

	if len(groups) == 1 {
		// Every resource entry addresses the same blob: upload the original payload.
		groups[0].data = data
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
		var blobName string
		if group.nameFormat == nil {
			blobName, err = e.generateBlobNameWithCompression(ops.signal, group.data)
			if err != nil {
				return fmt.Errorf("failed to generate blobname: %w", err)
			}
		} else {
			blobName = e.appendCompressionExtension(e.formatBlobName(*group.nameFormat))
		}
		return e.consumeData(ctx, blobName, data, ops.signal)
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
