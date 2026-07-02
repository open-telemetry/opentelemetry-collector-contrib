// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

func TestProcessorLookup(t *testing.T) {
	mappings := map[string]any{
		"key1":  "val1",
		"key2":  "val2",
		"rsrc1": "rsrcval1",
	}

	factory := NewFactoryWithOptions(WithSources(mockMapSourceFactory(mappings)))
	cfg := &Config{
		Source: SourceConfig{Type: "mockmap"},
		Lookups: []LookupConfig{
			{
				Key:     `log.attributes["k"]`,
				Context: ContextRecord,
				Attributes: []AttributeMapping{
					{Destination: "out", Default: "default"},
				},
			},
			{
				Key:     `resource.attributes["rk"]`,
				Context: ContextResource,
				Attributes: []AttributeMapping{
					{Destination: "rout"},
				},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("rk", "rsrc1")

	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty().Attributes().PutStr("k", "key1")
	sl.LogRecords().AppendEmpty().Attributes().PutStr("k", "missing")
	sl.LogRecords().AppendEmpty().Attributes().PutStr("k", "key2")

	err = proc.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	require.Len(t, sink.AllLogs(), 1)
	processedLogs := sink.AllLogs()[0]

	rout, ok := processedLogs.ResourceLogs().At(0).Resource().Attributes().Get("rout")
	assert.True(t, ok)
	assert.Equal(t, "rsrcval1", rout.Str())

	records := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 3, records.Len())

	out1, ok := records.At(0).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val1", out1.Str())

	out2, ok := records.At(1).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "default", out2.Str())

	out3, ok := records.At(2).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val2", out3.Str())
}

func TestProcessorTracesLookup(t *testing.T) {
	mappings := map[string]any{
		"key1":  "val1",
		"key2":  "val2",
		"rsrc1": "rsrcval1",
	}

	factory := NewFactoryWithOptions(WithSources(mockMapSourceFactory(mappings)))
	cfg := &Config{
		Source: SourceConfig{Type: "mockmap"},
		Lookups: []LookupConfig{
			{
				Key:     `span.attributes["k"]`,
				Context: ContextRecord,
				Attributes: []AttributeMapping{
					{Destination: "out", Default: "default"},
				},
			},
			{
				Key:     `resource.attributes["rk"]`,
				Context: ContextResource,
				Attributes: []AttributeMapping{
					{Destination: "rout"},
				},
			},
		},
	}

	sink := &consumertest.TracesSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateTraces(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("rk", "rsrc1")

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Spans().AppendEmpty().Attributes().PutStr("k", "key1")
	ss.Spans().AppendEmpty().Attributes().PutStr("k", "missing")
	ss.Spans().AppendEmpty().Attributes().PutStr("k", "key2")

	err = proc.ConsumeTraces(t.Context(), traces)
	require.NoError(t, err)

	require.Len(t, sink.AllTraces(), 1)
	processedTraces := sink.AllTraces()[0]

	rout, ok := processedTraces.ResourceSpans().At(0).Resource().Attributes().Get("rout")
	assert.True(t, ok)
	assert.Equal(t, "rsrcval1", rout.Str())

	spans := processedTraces.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
	require.Equal(t, 3, spans.Len())

	out1, ok := spans.At(0).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val1", out1.Str())

	out2, ok := spans.At(1).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "default", out2.Str())

	out3, ok := spans.At(2).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val2", out3.Str())
}

func TestProcessorMetricsLookup(t *testing.T) {
	mappings := map[string]any{
		"key1":  "val1",
		"key2":  "val2",
		"rsrc1": "rsrcval1",
	}

	factory := NewFactoryWithOptions(WithSources(mockMapSourceFactory(mappings)))
	cfg := &Config{
		Source: SourceConfig{Type: "mockmap"},
		Lookups: []LookupConfig{
			{
				Key:     `datapoint.attributes["k"]`,
				Context: ContextRecord,
				Attributes: []AttributeMapping{
					{Destination: "out", Default: "default"},
				},
			},
			{
				Key:     `resource.attributes["rk"]`,
				Context: ContextResource,
				Attributes: []AttributeMapping{
					{Destination: "rout"},
				},
			},
		},
	}

	sink := &consumertest.MetricsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateMetrics(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("rk", "rsrc1")
	sm := rm.ScopeMetrics().AppendEmpty()

	gaugeMetric := sm.Metrics().AppendEmpty()
	gaugeMetric.SetName("gauge")
	gaugeMetric.SetEmptyGauge().DataPoints().AppendEmpty().Attributes().PutStr("k", "key1")
	gaugeMetric.Gauge().DataPoints().AppendEmpty().Attributes().PutStr("k", "missing")

	sumMetric := sm.Metrics().AppendEmpty()
	sumMetric.SetName("sum")
	sumMetric.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("k", "key2")

	histMetric := sm.Metrics().AppendEmpty()
	histMetric.SetName("hist")
	histMetric.SetEmptyHistogram().DataPoints().AppendEmpty().Attributes().PutStr("k", "key1")

	expHistMetric := sm.Metrics().AppendEmpty()
	expHistMetric.SetName("exphist")
	expHistMetric.SetEmptyExponentialHistogram().DataPoints().AppendEmpty().Attributes().PutStr("k", "key2")

	summaryMetric := sm.Metrics().AppendEmpty()
	summaryMetric.SetName("summary")
	summaryMetric.SetEmptySummary().DataPoints().AppendEmpty().Attributes().PutStr("k", "key1")

	err = proc.ConsumeMetrics(t.Context(), md)
	require.NoError(t, err)

	require.Len(t, sink.AllMetrics(), 1)
	processedMetrics := sink.AllMetrics()[0]

	rout, ok := processedMetrics.ResourceMetrics().At(0).Resource().Attributes().Get("rout")
	assert.True(t, ok)
	assert.Equal(t, "rsrcval1", rout.Str())

	metrics := processedMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	require.Equal(t, 5, metrics.Len())

	out, ok := metrics.At(0).Gauge().DataPoints().At(0).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val1", out.Str())

	out, ok = metrics.At(0).Gauge().DataPoints().At(1).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "default", out.Str())

	out, ok = metrics.At(1).Sum().DataPoints().At(0).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val2", out.Str())

	out, ok = metrics.At(2).Histogram().DataPoints().At(0).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val1", out.Str())

	out, ok = metrics.At(3).ExponentialHistogram().DataPoints().At(0).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val2", out.Str())

	out, ok = metrics.At(4).Summary().DataPoints().At(0).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val1", out.Str())
}

func TestProcessorLookupError(t *testing.T) {
	errSource := lookupsource.NewSourceFactory(
		"errsource",
		func() lookupsource.SourceConfig { return &mockSourceConfig{} },
		func(_ context.Context, _ lookupsource.CreateSettings, _ lookupsource.SourceConfig) (lookupsource.Source, error) {
			return lookupsource.NewSource(
				func(_ context.Context, _ string) (any, bool, error) {
					return nil, false, errors.New("source error")
				},
				func() string { return "errsource" },
				nil,
				nil,
			), nil
		},
	)

	factory := NewFactoryWithOptions(WithSources(errSource))
	cfg := &Config{
		Source: SourceConfig{Type: "errsource"},
		Lookups: []LookupConfig{
			{
				Key:        `log.attributes["k"]`,
				Attributes: []AttributeMapping{{Destination: "out"}},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Attributes().PutStr("k", "anything")

	require.NoError(t, proc.ConsumeLogs(t.Context(), logs))

	records := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	_, ok := records.At(0).Attributes().Get("out")
	assert.False(t, ok, "out should not be set when lookup errors")
}

func TestProcessorMultipleResourcesAndScopes(t *testing.T) {
	mappings := map[string]any{
		"key1": "val1",
		"key2": "val2",
	}

	factory := NewFactoryWithOptions(WithSources(mockMapSourceFactory(mappings)))
	cfg := &Config{
		Source: SourceConfig{Type: "mockmap"},
		Lookups: []LookupConfig{
			{
				Key:        `log.attributes["k"]`,
				Attributes: []AttributeMapping{{Destination: "out"}},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	logs := plog.NewLogs()

	rl1 := logs.ResourceLogs().AppendEmpty()
	rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Attributes().PutStr("k", "key1")
	rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Attributes().PutStr("k", "key2")

	rl2 := logs.ResourceLogs().AppendEmpty()
	rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Attributes().PutStr("k", "key1")

	require.NoError(t, proc.ConsumeLogs(t.Context(), logs))

	processedLogs := sink.AllLogs()[0]
	require.Equal(t, 2, processedLogs.ResourceLogs().Len())

	out, ok := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val1", out.Str())

	out, ok = processedLogs.ResourceLogs().At(0).ScopeLogs().At(1).LogRecords().At(0).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val2", out.Str())

	out, ok = processedLogs.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("out")
	assert.True(t, ok)
	assert.Equal(t, "val1", out.Str())
}

func TestProcessorMapResult(t *testing.T) {
	// Source returns map[string]any for 1:N lookups
	mapSource := lookupsource.NewSourceFactory(
		"mapresult",
		func() lookupsource.SourceConfig { return &mockSourceConfig{} },
		func(_ context.Context, _ lookupsource.CreateSettings, _ lookupsource.SourceConfig) (lookupsource.Source, error) {
			return lookupsource.NewSource(
				func(_ context.Context, key string) (any, bool, error) {
					if key == "user001" {
						return map[string]any{
							"name":  "Alice Johnson",
							"email": "alice@example.com",
							"role":  "admin",
						}, true, nil
					}
					return nil, false, nil
				},
				func() string { return "mapresult" },
				nil,
				nil,
			), nil
		},
	)

	factory := NewFactoryWithOptions(WithSources(mapSource))
	cfg := &Config{
		Source: SourceConfig{Type: "mapresult"},
		Lookups: []LookupConfig{
			{
				Key: `log.attributes["user.id"]`,
				Attributes: []AttributeMapping{
					{Source: "name", Destination: "user.name", Default: "Unknown"},
					{Source: "email", Destination: "user.email"},
					{Source: "role", Destination: "user.role", Context: ContextResource},
				},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	// Test with found key
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Attributes().PutStr("user.id", "user001")

	err = proc.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	processedLogs := sink.AllLogs()[0]
	record := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	resourceAttrs := processedLogs.ResourceLogs().At(0).Resource().Attributes()

	name, ok := record.Attributes().Get("user.name")
	assert.True(t, ok)
	assert.Equal(t, "Alice Johnson", name.Str())

	email, ok := record.Attributes().Get("user.email")
	assert.True(t, ok)
	assert.Equal(t, "alice@example.com", email.Str())

	role, ok := resourceAttrs.Get("user.role")
	assert.True(t, ok)
	assert.Equal(t, "admin", role.Str())
}

func TestProcessorMapResultNotFound(t *testing.T) {
	mapSource := lookupsource.NewSourceFactory(
		"mapresult",
		func() lookupsource.SourceConfig { return &mockSourceConfig{} },
		func(_ context.Context, _ lookupsource.CreateSettings, _ lookupsource.SourceConfig) (lookupsource.Source, error) {
			return lookupsource.NewSource(
				func(_ context.Context, _ string) (any, bool, error) {
					return nil, false, nil
				},
				func() string { return "mapresult" },
				nil,
				nil,
			), nil
		},
	)

	factory := NewFactoryWithOptions(WithSources(mapSource))
	cfg := &Config{
		Source: SourceConfig{Type: "mapresult"},
		Lookups: []LookupConfig{
			{
				Key: `log.attributes["user.id"]`,
				Attributes: []AttributeMapping{
					{Source: "name", Destination: "user.name", Default: "Unknown"},
					{Source: "email", Destination: "user.email"},
				},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Attributes().PutStr("user.id", "unknown-user")

	err = proc.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	processedLogs := sink.AllLogs()[0]
	record := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	// Default should be applied for "name"
	name, ok := record.Attributes().Get("user.name")
	assert.True(t, ok)
	assert.Equal(t, "Unknown", name.Str())

	// No default for "email" — should not be set
	_, ok = record.Attributes().Get("user.email")
	assert.False(t, ok)
}

func TestProcessorNoSourceAttribute(t *testing.T) {
	factory := NewFactoryWithOptions(WithSources(mockSourceFactory("found")))

	cfg := &Config{
		Source: SourceConfig{Type: "mock"},
		Lookups: []LookupConfig{
			{
				Key: `log.attributes["nonexistent"]`,
				Attributes: []AttributeMapping{
					{Destination: "result"},
				},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	// Create test log without the source attribute
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Attributes().PutStr("other", "value")

	err = proc.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// Result should not be set since source attribute doesn't exist
	processedLogs := sink.AllLogs()[0]
	_, exists := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("result")
	assert.False(t, exists, "result should not be set when source attribute is missing")
}

func TestProcessorValueTypes(t *testing.T) {
	type unsupportedType struct{ field string }

	mappings := map[string]any{
		"int-key":    int64(42),
		"float-key":  float64(3.14),
		"bool-key":   true,
		"bytes-key":  []byte("raw bytes"),
		"slice-key":  []any{"a", "b", "c"},
		"struct-key": unsupportedType{field: "test"},
	}

	factory := NewFactoryWithOptions(WithSources(mockMapSourceFactory(mappings)))
	cfg := &Config{
		Source: SourceConfig{Type: "mockmap"},
		Lookups: []LookupConfig{
			{
				Key: `log.attributes["key"]`,
				Attributes: []AttributeMapping{
					{Destination: "result"},
				},
			},
		},
	}

	sink := &consumertest.LogsSink{}
	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	defer func() { _ = proc.Shutdown(t.Context()) }()

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	for _, key := range []string{"int-key", "float-key", "bool-key", "bytes-key", "slice-key", "struct-key"} {
		lr := sl.LogRecords().AppendEmpty()
		lr.Attributes().PutStr("key", key)
	}

	err = proc.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	require.Len(t, sink.AllLogs(), 1)
	records := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	require.Equal(t, 6, records.Len())

	// int64 => IntValue
	val, ok := records.At(0).Attributes().Get("result")
	assert.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeInt, val.Type())
	assert.Equal(t, int64(42), val.Int())

	// float64 => DoubleValue
	val, ok = records.At(1).Attributes().Get("result")
	assert.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeDouble, val.Type())
	assert.Equal(t, float64(3.14), val.Double())

	// bool => BoolValue
	val, ok = records.At(2).Attributes().Get("result")
	assert.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeBool, val.Type())
	assert.True(t, val.Bool())

	// []byte => BytesValue
	val, ok = records.At(3).Attributes().Get("result")
	assert.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeBytes, val.Type())
	assert.Equal(t, []byte("raw bytes"), val.Bytes().AsRaw())

	// []any => SliceValue
	val, ok = records.At(4).Attributes().Get("result")
	assert.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeSlice, val.Type())
	assert.Equal(t, 3, val.Slice().Len())

	// unsupported type => stringify fallback
	val, ok = records.At(5).Attributes().Get("result")
	assert.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeStr, val.Type())
	assert.Equal(t, "{test}", val.Str())
}

// mockSourceFactory creates a source factory that always returns the given value.
func mockSourceFactory(value string) lookupsource.SourceFactory {
	return lookupsource.NewSourceFactory(
		"mock",
		func() lookupsource.SourceConfig { return &mockSourceConfig{} },
		func(_ context.Context, _ lookupsource.CreateSettings, _ lookupsource.SourceConfig) (lookupsource.Source, error) {
			return lookupsource.NewSource(
				func(_ context.Context, _ string) (any, bool, error) {
					return value, true, nil
				},
				func() string { return "mock" },
				nil,
				nil,
			), nil
		},
	)
}

// mockMapSourceFactory creates a source factory that looks up values from a map.
func mockMapSourceFactory(mappings map[string]any) lookupsource.SourceFactory {
	return lookupsource.NewSourceFactory(
		"mockmap",
		func() lookupsource.SourceConfig { return &mockSourceConfig{} },
		func(_ context.Context, _ lookupsource.CreateSettings, _ lookupsource.SourceConfig) (lookupsource.Source, error) {
			return lookupsource.NewSource(
				func(_ context.Context, key string) (any, bool, error) {
					val, found := mappings[key]
					return val, found, nil
				},
				func() string { return "mockmap" },
				nil,
				nil,
			), nil
		},
	)
}
