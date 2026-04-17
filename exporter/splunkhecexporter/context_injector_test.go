// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	collclient "go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// ctxCaptureLogs captures the context passed to ConsumeLogs.
type ctxCaptureLogs struct{ ctx context.Context }

func (_ *ctxCaptureLogs) Capabilities() consumer.Capabilities { return consumer.Capabilities{} }
func (c *ctxCaptureLogs) ConsumeLogs(ctx context.Context, _ plog.Logs) error {
	c.ctx = ctx
	return nil
}

// ctxCaptureMetrics captures the context passed to ConsumeMetrics.
type ctxCaptureMetrics struct{ ctx context.Context }

func (_ *ctxCaptureMetrics) Capabilities() consumer.Capabilities { return consumer.Capabilities{} }
func (c *ctxCaptureMetrics) ConsumeMetrics(ctx context.Context, _ pmetric.Metrics) error {
	c.ctx = ctx
	return nil
}

// ctxCaptureTraces captures the context passed to ConsumeTraces.
type ctxCaptureTraces struct{ ctx context.Context }

func (_ *ctxCaptureTraces) Capabilities() consumer.Capabilities { return consumer.Capabilities{} }
func (c *ctxCaptureTraces) ConsumeTraces(ctx context.Context, _ ptrace.Traces) error {
	c.ctx = ctx
	return nil
}

func TestInjectHECMetadata(t *testing.T) {
	t.Run("both keys present", func(t *testing.T) {
		attrs := plog.NewLogs().ResourceLogs().AppendEmpty().Resource().Attributes()
		attrs.PutStr(splunk.HecTokenLabel, "my-token")
		attrs.PutStr(splunk.DefaultIndexLabel, "my-index")

		ctx := injectHECMetadata(t.Context(), attrs)

		meta := collclient.FromContext(ctx).Metadata
		assert.Equal(t, []string{"my-token"}, meta.Get(splunk.HecTokenLabel))
		assert.Equal(t, []string{"my-index"}, meta.Get(splunk.DefaultIndexLabel))
	})

	t.Run("token only", func(t *testing.T) {
		attrs := plog.NewLogs().ResourceLogs().AppendEmpty().Resource().Attributes()
		attrs.PutStr(splunk.HecTokenLabel, "tok")

		ctx := injectHECMetadata(t.Context(), attrs)

		meta := collclient.FromContext(ctx).Metadata
		assert.Equal(t, []string{"tok"}, meta.Get(splunk.HecTokenLabel))
		assert.Empty(t, meta.Get(splunk.DefaultIndexLabel))
	})

	t.Run("no keys returns original context", func(t *testing.T) {
		attrs := plog.NewLogs().ResourceLogs().AppendEmpty().Resource().Attributes()
		parent := t.Context()
		got := injectHECMetadata(parent, attrs)
		assert.Equal(t, parent, got)
	})
}

func TestLogsHECMetadataInjector(t *testing.T) {
	capture := &ctxCaptureLogs{}
	inj := &logsHECMetadataInjector{next: capture}

	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr(splunk.HecTokenLabel, "injected-token")

	require.NoError(t, inj.ConsumeLogs(t.Context(), ld))
	meta := collclient.FromContext(capture.ctx).Metadata
	assert.Equal(t, []string{"injected-token"}, meta.Get(splunk.HecTokenLabel))
}

func TestMetricsHECMetadataInjector(t *testing.T) {
	capture := &ctxCaptureMetrics{}
	inj := &metricsHECMetadataInjector{next: capture}

	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr(splunk.DefaultIndexLabel, "idx")

	require.NoError(t, inj.ConsumeMetrics(t.Context(), md))
	meta := collclient.FromContext(capture.ctx).Metadata
	assert.Equal(t, []string{"idx"}, meta.Get(splunk.DefaultIndexLabel))
}

func TestTracesHECMetadataInjector(t *testing.T) {
	capture := &ctxCaptureTraces{}
	inj := &tracesHECMetadataInjector{next: capture}

	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().Resource().Attributes().PutStr(splunk.HecTokenLabel, "trace-token")

	require.NoError(t, inj.ConsumeTraces(t.Context(), td))
	meta := collclient.FromContext(capture.ctx).Metadata
	assert.Equal(t, []string{"trace-token"}, meta.Get(splunk.HecTokenLabel))
}

func TestLogsHECMetadataInjector_EmptyLogs(t *testing.T) {
	capture := &ctxCaptureLogs{}
	inj := &logsHECMetadataInjector{next: capture}

	require.NoError(t, inj.ConsumeLogs(t.Context(), plog.NewLogs()))
	// context should be unchanged — no resource means no metadata injected
	assert.Empty(t, collclient.FromContext(capture.ctx).Metadata.Get(splunk.HecTokenLabel))
}
