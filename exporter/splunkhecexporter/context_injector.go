// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"

	collclient "go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// logsHECMetadataInjector sits between batchperresourceattr and the
// exporterhelper-wrapped exporter. After batchperresourceattr has split the
// batch into uniform-token/index subsets, this wrapper reads those values from
// the first resource and injects them as client.Metadata so the batcher's
// MetadataKeys partitioner keeps them in separate partitions.
type logsHECMetadataInjector struct {
	next consumer.Logs
}

func (l *logsHECMetadataInjector) Capabilities() consumer.Capabilities {
	return l.next.Capabilities()
}

func (l *logsHECMetadataInjector) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if ld.ResourceLogs().Len() > 0 {
		ctx = injectHECMetadata(ctx, ld.ResourceLogs().At(0).Resource().Attributes())
	}
	return l.next.ConsumeLogs(ctx, ld)
}

type metricsHECMetadataInjector struct {
	next consumer.Metrics
}

func (m *metricsHECMetadataInjector) Capabilities() consumer.Capabilities {
	return m.next.Capabilities()
}

func (m *metricsHECMetadataInjector) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if md.ResourceMetrics().Len() > 0 {
		ctx = injectHECMetadata(ctx, md.ResourceMetrics().At(0).Resource().Attributes())
	}
	return m.next.ConsumeMetrics(ctx, md)
}

type tracesHECMetadataInjector struct {
	next consumer.Traces
}

func (t *tracesHECMetadataInjector) Capabilities() consumer.Capabilities {
	return t.next.Capabilities()
}

func (t *tracesHECMetadataInjector) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if td.ResourceSpans().Len() > 0 {
		ctx = injectHECMetadata(ctx, td.ResourceSpans().At(0).Resource().Attributes())
	}
	return t.next.ConsumeTraces(ctx, td)
}

// injectHECMetadata reads HecTokenLabel and DefaultIndexLabel from attrs and
// injects them as client.Metadata into ctx. Returns ctx unchanged if neither
// key is present.
func injectHECMetadata(ctx context.Context, attrs pcommon.Map) context.Context {
	meta := map[string][]string{}
	if v, ok := attrs.Get(splunk.HecTokenLabel); ok {
		meta[splunk.HecTokenLabel] = []string{v.Str()}
	}
	if v, ok := attrs.Get(splunk.DefaultIndexLabel); ok {
		meta[splunk.DefaultIndexLabel] = []string{v.Str()}
	}
	if len(meta) == 0 {
		return ctx
	}
	return collclient.NewContext(ctx, collclient.Info{Metadata: collclient.NewMetadata(meta)})
}
