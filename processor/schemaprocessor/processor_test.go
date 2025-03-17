// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemaprocessor

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap/zaptest"
)

type dummySchemaProvider struct {
	transformations string
}

func (m *dummySchemaProvider) Retrieve(_ context.Context, _ string) (string, error) {
	transformations := strings.ReplaceAll(m.transformations, "\t", "    ")
	data := fmt.Sprintf(`
file_format: 1.1.0
schema_url: http://opentelemetry.io/schemas/1.9.0
versions:%s`, transformations)

	data = strings.TrimSpace(data)
	return data, nil
}

func newTestSchemaProcessor(t *testing.T, transformations string, targerVerion string) *schemaProcessor {
	cfg := &Config{
		Targets: []string{fmt.Sprintf("http://opentelemetry.io/schemas/%s", targerVerion)},
	}
	trans, err := newSchemaProcessor(context.Background(), cfg, processor.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zaptest.NewLogger(t),
		},
	})
	require.NoError(t, err, "Must not error when creating default schemaProcessor")
	trans.manager.AddProvider(&dummySchemaProvider{
		transformations: transformations,
	})
	return trans
}

func TestSchemaProcessorStart(t *testing.T) {
	t.Parallel()

	trans := newTestSchemaProcessor(t, "", "1.9.0")
	assert.NoError(t, trans.start(context.Background(), nil))
}

func TestSchemaProcessorProcessing(t *testing.T) {
	t.Parallel()
	// these tests are just to ensure that the processor does not error out
	// and that the data is not modified as dummyprovider has no transformations

	trans := newTestSchemaProcessor(t, "", "1.9.0")
	t.Run("metrics", func(t *testing.T) {
		in := pmetric.NewMetrics()
		in.ResourceMetrics().AppendEmpty()
		in.ResourceMetrics().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
		in.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty()
		m := in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
		m.SetName("test-data")
		m.SetDescription("Only used throughout tests")
		m.SetUnit("seconds")
		m.CopyTo(in.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0))

		out, err := trans.processMetrics(context.Background(), in)
		assert.NoError(t, err, "Must not error when processing metrics")
		assert.Equal(t, in, out, "Must return the same data")
	})

	t.Run("traces", func(t *testing.T) {
		in := ptrace.NewTraces()
		in.ResourceSpans().AppendEmpty()
		in.ResourceSpans().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
		in.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
		s := in.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()
		s.SetName("http.request")
		s.SetKind(ptrace.SpanKindConsumer)
		s.SetSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7})
		s.CopyTo(in.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0))

		out, err := trans.processTraces(context.Background(), in)
		assert.NoError(t, err, "Must not error when processing traces")
		assert.Equal(t, in, out, "Must return the same data")
	})

	t.Run("logs", func(t *testing.T) {
		in := plog.NewLogs()
		in.ResourceLogs().AppendEmpty()
		in.ResourceLogs().At(0).SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
		in.ResourceLogs().At(0).ScopeLogs().AppendEmpty()
		l := in.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
		l.SetName("magical-logs")
		l.SetVersion("alpha")
		l.CopyTo(in.ResourceLogs().At(0).ScopeLogs().At(0).Scope())

		out, err := trans.processLogs(context.Background(), in)
		assert.NoError(t, err, "Must not error when processing metrics")
		assert.Equal(t, in, out, "Must return the same data")
	})
}
