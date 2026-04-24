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
	"go.opentelemetry.io/collector/component/componenttest"
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

func newTestSchemaProcessorWithMigration(t *testing.T, transformations, targetVersion string, migration []MigrationEntry) *schemaProcessor {
	cfg := &Config{
		Targets:   []string{fmt.Sprintf("http://opentelemetry.io/schemas/%s", targetVersion)},
		Migration: migration,
	}
	trans, err := newSchemaProcessor(t.Context(), cfg, processor.Settings{
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

func newTestSchemaProcessor(t *testing.T, transformations, targerVerion string) *schemaProcessor {
	cfg := &Config{
		Targets: []string{fmt.Sprintf("http://opentelemetry.io/schemas/%s", targerVerion)},
	}
	trans, err := newSchemaProcessor(t.Context(), cfg, processor.Settings{
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
	assert.NoError(t, trans.start(t.Context(), componenttest.NewNopHost()))
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

		out, err := trans.processMetrics(t.Context(), in)
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

		out, err := trans.processTraces(t.Context(), in)
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

		out, err := trans.processLogs(t.Context(), in)
		assert.NoError(t, err, "Must not error when processing metrics")
		assert.Equal(t, in, out, "Must return the same data")
	})
}

func TestMigrationModePreservesAttributes(t *testing.T) {
	t.Parallel()

	// Schema renames service_version -> service.version in version 1.9.0.
	// Version 1.8.0 must be present so the translator recognizes it.
	transformations := `
  1.9.0:
    all:
      changes:
        - rename_attributes:
            attribute_map:
              service_version: service.version
  1.8.0:`

	t.Run("upgrade with migration preserves both attributes", func(t *testing.T) {
		trans := newTestSchemaProcessorWithMigration(t, transformations, "1.9.0",
			[]MigrationEntry{{Target: "http://opentelemetry.io/schemas/1.9.0", From: "http://opentelemetry.io/schemas/1.8.0"}})

		in := plog.NewLogs()
		rl := in.ResourceLogs().AppendEmpty()
		rl.SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
		rl.Resource().Attributes().PutStr("service_version", "v1.0")

		out, err := trans.processLogs(t.Context(), in)
		require.NoError(t, err)

		attrs := out.ResourceLogs().At(0).Resource().Attributes()
		// Both old and new attribute names should be present
		oldVal, oldExists := attrs.Get("service_version")
		newVal, newExists := attrs.Get("service.version")
		assert.True(t, oldExists, "original attribute should be preserved")
		assert.True(t, newExists, "renamed attribute should be added")
		assert.Equal(t, "v1.0", oldVal.Str())
		assert.Equal(t, "v1.0", newVal.Str())
	})

	t.Run("upgrade without migration only has new attribute", func(t *testing.T) {
		trans := newTestSchemaProcessor(t, transformations, "1.9.0")

		in := plog.NewLogs()
		rl := in.ResourceLogs().AppendEmpty()
		rl.SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
		rl.Resource().Attributes().PutStr("service_version", "v1.0")

		out, err := trans.processLogs(t.Context(), in)
		require.NoError(t, err)

		attrs := out.ResourceLogs().At(0).Resource().Attributes()
		_, oldExists := attrs.Get("service_version")
		newVal, newExists := attrs.Get("service.version")
		assert.False(t, oldExists, "original attribute should be removed")
		assert.True(t, newExists, "renamed attribute should be present")
		assert.Equal(t, "v1.0", newVal.Str())
	})

	t.Run("both attributes already exist are preserved", func(t *testing.T) {
		trans := newTestSchemaProcessorWithMigration(t, transformations, "1.9.0",
			[]MigrationEntry{{Target: "http://opentelemetry.io/schemas/1.9.0", From: "http://opentelemetry.io/schemas/1.8.0"}})

		in := plog.NewLogs()
		rl := in.ResourceLogs().AppendEmpty()
		rl.SetSchemaUrl("http://opentelemetry.io/schemas/1.8.0")
		rl.Resource().Attributes().PutStr("service_version", "v1.0")
		rl.Resource().Attributes().PutStr("service.version", "v2.0")

		out, err := trans.processLogs(t.Context(), in)
		require.NoError(t, err)

		attrs := out.ResourceLogs().At(0).Resource().Attributes()
		oldVal, _ := attrs.Get("service_version")
		newVal, _ := attrs.Get("service.version")
		assert.Equal(t, "v1.0", oldVal.Str(), "original value preserved")
		assert.Equal(t, "v2.0", newVal.Str(), "existing target value preserved")
	})

	t.Run("downgrade with migration preserves both attributes", func(t *testing.T) {
		// Target is 1.8.0 (downgrade), signal arrives at 1.9.0.
		// Revision 1.9.0 renamed service_version -> service.version.
		// Rollback should undo that rename but copy mode preserves both.
		trans := newTestSchemaProcessorWithMigration(t, transformations, "1.8.0",
			[]MigrationEntry{{Target: "http://opentelemetry.io/schemas/1.8.0", From: "http://opentelemetry.io/schemas/1.9.0"}})

		in := plog.NewLogs()
		rl := in.ResourceLogs().AppendEmpty()
		rl.SetSchemaUrl("http://opentelemetry.io/schemas/1.9.0")
		rl.Resource().Attributes().PutStr("service.version", "v1.0")

		out, err := trans.processLogs(t.Context(), in)
		require.NoError(t, err)

		attrs := out.ResourceLogs().At(0).Resource().Attributes()
		oldVal, oldExists := attrs.Get("service.version")
		newVal, newExists := attrs.Get("service_version")
		assert.True(t, oldExists, "original attribute should be preserved")
		assert.True(t, newExists, "rolled-back attribute should be added")
		assert.Equal(t, "v1.0", oldVal.Str())
		assert.Equal(t, "v1.0", newVal.Str())
	})
}
