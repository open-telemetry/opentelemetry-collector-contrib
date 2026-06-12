// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

func TestNewRevisionFromV2Resolved_BuildsRenameMaps(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(LoadTranslationVersion(t, "v2_resolved_simple.yaml"))
	require.NoError(t, err)
	require.Equal(t, SchemaFormatV2Resolved, parsed.Format)

	ver := &Version{2, 0, 0}
	rev := NewRevisionFromV2Resolved(ver, parsed.V2Resolved, false)
	require.NotNil(t, rev)
	assert.Equal(t, ver, rev.Version())

	// Attribute rename in `all` ChangeList.
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("old.attr.name", "value")
	require.NoError(t, rev.all.Apply(resource))
	got, ok := resource.Attributes().Get("new.attr.name")
	require.True(t, ok, "old.attr.name should be renamed to new.attr.name")
	assert.Equal(t, "value", got.AsString())

	// Metric rename.
	metric := pmetric.NewMetric()
	metric.SetName("old.sum.metric")
	require.NoError(t, rev.metrics.Apply(metric))
	assert.Equal(t, "new.sum.metric", metric.Name())

	// Span rename.
	span := ptrace.NewSpan()
	span.SetName("http.server.old")
	require.NoError(t, rev.spans.Apply(span))
	assert.Equal(t, "http.server.request", span.Name())

	// Event rename (handled at the parent span; iterates events).
	parent := ptrace.NewSpan()
	ev := parent.Events().AppendEmpty()
	ev.SetName("old_stack_trace")
	require.NoError(t, rev.spanEvents.Apply(parent))
	assert.Equal(t, "stack_trace", parent.Events().At(0).Name())
}

func TestNewRevisionFromV2Resolved_NoRenames(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(LoadTranslationVersion(t, "v2_resolved_norenames.yaml"))
	require.NoError(t, err)
	rev := NewRevisionFromV2Resolved(&Version{2, 0, 0}, parsed.V2Resolved, false)

	// `obsoleted` deprecations should not produce migrators.
	assert.Empty(t, rev.all.Migrators)
	assert.Empty(t, rev.metrics.Migrators)
	assert.Empty(t, rev.spans.Migrators)
	assert.Empty(t, rev.spanEvents.Migrators)
}

func TestV2Translator_MigrationModePreservesOldAttribute(t *testing.T) {
	t.Parallel()

	yaml := LoadTranslationVersion(t, "v2_resolved_simple.yaml")
	from := &Version{1, 30, 0}
	tn, err := newTranslator(zaptest.NewLogger(t), "https://opentelemetry.io/schemas/2.0.0", yaml, from)
	require.NoError(t, err)

	rl := plog.NewResourceLogs()
	rl.Resource().Attributes().PutStr("old.attr.name", "value")
	rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.30.0")
	require.NoError(t, tn.ApplyAllResourceChanges(rl, rl.SchemaUrl()))

	_, hasNew := rl.Resource().Attributes().Get("new.attr.name")
	_, hasOld := rl.Resource().Attributes().Get("old.attr.name")
	assert.True(t, hasNew, "renamed attribute must be present")
	assert.True(t, hasOld, "migration mode must preserve the old attribute")
}

func TestV2Translator_SingleHopApply(t *testing.T) {
	t.Parallel()

	log := zaptest.NewLogger(t)
	yaml := LoadTranslationVersion(t, "v2_resolved_simple.yaml")
	_ = pcommon.NewResource // satisfy import if unused below

	// Upgrade: incoming 1.30.0, target 2.0.0 -> apply direction.
	{
		tn, err := newTranslator(log, "https://opentelemetry.io/schemas/2.0.0", yaml, nil)
		require.NoError(t, err)
		rl := plog.NewResourceLogs()
		rl.Resource().Attributes().PutStr("old.attr.name", "value")
		rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.30.0")
		require.NoError(t, tn.ApplyAllResourceChanges(rl, rl.SchemaUrl()))
		_, ok := rl.Resource().Attributes().Get("new.attr.name")
		assert.True(t, ok, "upgrade should rename old -> new")
		assert.Equal(t, "https://opentelemetry.io/schemas/2.0.0", rl.SchemaUrl())
	}

	// Downgrade: incoming 2.0.0, target 1.30.0 -> revert direction (uses
	// reversed map built from the v2 resolved registry).
	{
		tn, err := newTranslator(log, "https://opentelemetry.io/schemas/1.30.0", yaml, nil)
		require.NoError(t, err)
		rl := plog.NewResourceLogs()
		rl.Resource().Attributes().PutStr("new.attr.name", "value")
		rl.SetSchemaUrl("https://opentelemetry.io/schemas/2.0.0")
		require.NoError(t, tn.ApplyAllResourceChanges(rl, rl.SchemaUrl()))
		_, ok := rl.Resource().Attributes().Get("old.attr.name")
		assert.True(t, ok, "downgrade should rename new -> old via reversed map")
		assert.Equal(t, "https://opentelemetry.io/schemas/1.30.0", rl.SchemaUrl())
	}
}
