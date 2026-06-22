// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package profiles

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/config"
	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

type mockProfileExporter struct {
	profiles    []pprofile.Profiles
	exportCalls []pprofile.Profiles
}

func (m *mockProfileExporter) Export(_ context.Context, td pprofile.Profiles) error {
	cp := pprofile.NewProfiles()
	td.CopyTo(cp)
	m.profiles = append(m.profiles, cp)
	m.exportCalls = append(m.exportCalls, cp)
	return nil
}

func (*mockProfileExporter) Shutdown(context.Context) error {
	return nil
}

type errorProfileExporter struct {
	err         error
	exportCalls int
}

func (e *errorProfileExporter) Export(_ context.Context, _ pprofile.Profiles) error {
	e.exportCalls++
	return e.err
}

func (*errorProfileExporter) Shutdown(context.Context) error {
	return nil
}

func countProfileRecords(exported []pprofile.Profiles) int {
	total := 0
	for _, p := range exported {
		rps := p.ResourceProfiles()
		for i := 0; i < rps.Len(); i++ {
			sps := rps.At(i).ScopeProfiles()
			for j := 0; j < sps.Len(); j++ {
				total += sps.At(j).Profiles().Len()
			}
		}
	}
	return total
}

func findProfileAttribute(t *testing.T, dict pprofile.ProfilesDictionary, key string) pcommon.Value {
	t.Helper()

	st := dict.StringTable()
	attrTable := dict.AttributeTable()
	for i := 0; i < attrTable.Len(); i++ {
		attr := attrTable.At(i)
		if st.At(int(attr.KeyStrindex())) == key {
			return attr.Value()
		}
	}

	t.Fatalf("attribute %q not found", key)
	return pcommon.NewValueEmpty()
}

func TestFixedNumberOfProfiles(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
		},
		NumProfiles:     5,
		SampleCount:     2,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	time.Sleep(1 * time.Second)

	assert.Equal(t, 5, countProfileRecords(m.profiles))
}

func TestRateOfProfiles(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			Rate:          10,
			TotalDuration: types.DurationWithInf(time.Second / 2),
			WorkerCount:   1,
		},
		SampleCount:     2,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}
	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	require.NoError(t, run(cfg, exporterFactory, zap.NewNop()))

	count := countProfileRecords(m.profiles)
	assert.GreaterOrEqual(t, count, 5, "there should have been 5 or more profiles, had %d", count)
	assert.LessOrEqual(t, count, 20, "there should have been less than 20 profiles, had %d", count)
}

func TestUnthrottled(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			TotalDuration: types.DurationWithInf(1 * time.Second),
			WorkerCount:   1,
		},
		SampleCount:     2,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}
	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	count := countProfileRecords(m.profiles)
	assert.Greater(t, count, 100, "there should have been more than 100 profiles, had %d", count)
}

func TestDurationInf(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			TotalDuration: types.DurationWithInf(-1),
			WorkerCount:   1,
		},
		NumProfiles:     1,
		SampleCount:     2,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}

	require.True(t, cfg.TotalDuration.IsInf())
	require.NoError(t, cfg.Validate())
}

func TestProfileStructure(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
		},
		NumProfiles:     1,
		SampleCount:     5,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 30 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	require.Equal(t, 1, countProfileRecords(m.profiles))

	p := m.profiles[0]
	dict := p.Dictionary()
	profile := p.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0)

	// ProfileID should be set (non-zero)
	var zeroID pprofile.ProfileID
	assert.NotEqual(t, zeroID, profile.ProfileID())

	// DurationNano should match config
	assert.Equal(t, uint64((30 * time.Second).Nanoseconds()), profile.DurationNano())

	// SampleType should be cpu/nanoseconds
	st := dict.StringTable()
	sampleType := profile.SampleType()
	assert.Equal(t, "cpu", st.At(int(sampleType.TypeStrindex())))
	assert.Equal(t, "nanoseconds", st.At(int(sampleType.UnitStrindex())))

	// PeriodType should be cpu/nanoseconds
	periodType := profile.PeriodType()
	assert.Equal(t, "cpu", st.At(int(periodType.TypeStrindex())))
	assert.Equal(t, "nanoseconds", st.At(int(periodType.UnitStrindex())))

	// Period should be 10ms in nanoseconds
	assert.Equal(t, int64(10000000), profile.Period())

	// Sample count should match
	assert.Equal(t, 5, profile.Samples().Len())

	// Each sample should have a value
	for i := 0; i < profile.Samples().Len(); i++ {
		sample := profile.Samples().At(i)
		assert.Equal(t, 1, sample.Values().Len(), "each sample should have exactly 1 value")
	}

	// Time should be set (non-zero)
	assert.NotZero(t, profile.Time())
}

func TestStackDepth(t *testing.T) {
	depth := 7
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
		},
		NumProfiles:     1,
		SampleCount:     3,
		StackDepth:      depth,
		UniqueFunctions: 20,
		ProfileDuration: 10 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	require.Equal(t, 1, countProfileRecords(m.profiles))

	p := m.profiles[0]
	dict := p.Dictionary()

	// Each stack should have `depth` location indices. Index 0 is the reserved
	// zero-value stack required by the OTLP profiles spec, so it is skipped.
	stackTable := dict.StackTable()
	for i := 1; i < stackTable.Len(); i++ {
		stack := stackTable.At(i)
		assert.Equal(t, depth, stack.LocationIndices().Len(),
			"stack %d should have %d location indices", i, depth)
	}
}

func TestUniqueFunctions(t *testing.T) {
	uniqueFuncs := 15
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
		},
		NumProfiles:     1,
		SampleCount:     3,
		StackDepth:      3,
		UniqueFunctions: uniqueFuncs,
		ProfileDuration: 10 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	require.Equal(t, 1, countProfileRecords(m.profiles))

	p := m.profiles[0]
	dict := p.Dictionary()

	// The function table holds uniqueFuncs real functions plus the reserved
	// zero-value entry at index 0 required by the OTLP profiles spec.
	assert.Equal(t, uniqueFuncs+1, dict.FunctionTable().Len(),
		"function table should have %d functions plus the zero-value entry", uniqueFuncs)
}

func TestProfilesWithTraceCorrelation(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
		},
		NumProfiles:     1,
		SampleCount:     3,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
		TraceID:         "ae87dadd90e9935a4bc9660628efd569",
		SpanID:          "5828fa4960140870",
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	require.Equal(t, 1, countProfileRecords(m.profiles))

	p := m.profiles[0]
	dict := p.Dictionary()
	profile := p.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0)

	// LinkTable should be populated
	assert.Positive(t, dict.LinkTable().Len(), "link table should not be empty")

	// Find the real link (non-zero TraceID)
	foundLink := false
	for i := 0; i < dict.LinkTable().Len(); i++ {
		link := dict.LinkTable().At(i)
		if link.TraceID().String() == "ae87dadd90e9935a4bc9660628efd569" {
			foundLink = true
			assert.Equal(t, "5828fa4960140870", link.SpanID().String())
		}
	}
	assert.True(t, foundLink, "should find a link with the configured TraceID")

	// All samples should have LinkIndex > 0 (pointing to the real link, not dummy)
	for i := 0; i < profile.Samples().Len(); i++ {
		sample := profile.Samples().At(i)
		assert.Positive(t, sample.LinkIndex(),
			"sample %d should have LinkIndex > 0", i)
	}
}

func TestProfilesWithSpanOnlyCorrelation(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
		},
		NumProfiles:     1,
		SampleCount:     3,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
		SpanID:          "5828fa4960140870",
		// Intentionally omitted TraceID
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	require.Equal(t, 1, countProfileRecords(m.profiles))

	p := m.profiles[0]
	dict := p.Dictionary()
	profile := p.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0)

	assert.Positive(t, dict.LinkTable().Len(), "link table should not be empty")

	foundLink := false
	for i := 0; i < dict.LinkTable().Len(); i++ {
		link := dict.LinkTable().At(i)
		if link.SpanID().String() == "5828fa4960140870" {
			foundLink = true
			assert.Equal(t, pcommon.TraceID{}.String(), link.TraceID().String())
		}
	}
	assert.True(t, foundLink, "should find a link with the configured SpanID")

	for i := 0; i < profile.Samples().Len(); i++ {
		sample := profile.Samples().At(i)
		assert.Positive(t, sample.LinkIndex(),
			"sample %d should have LinkIndex > 0", i)
	}
}

func TestProfilesWithNoTraceCorrelation(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
		},
		NumProfiles:     1,
		SampleCount:     3,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	require.Equal(t, 1, countProfileRecords(m.profiles))

	p := m.profiles[0]
	dict := p.Dictionary()
	profile := p.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0)

	// LinkTable should contain only the reserved zero-value entry at index 0.
	assert.Equal(t, 1, dict.LinkTable().Len(), "link table should hold only the zero-value entry")

	// All samples should have LinkIndex == 0
	for i := 0; i < profile.Samples().Len(); i++ {
		sample := profile.Samples().At(i)
		assert.Equal(t, int32(0), sample.LinkIndex(),
			"sample %d should have LinkIndex == 0", i)
	}
}

func TestProfilesWithTelemetryAttributes(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount:         1,
			TelemetryAttributes: config.KeyValue{"k1": "v1", "k2": "v2"},
		},
		NumProfiles:     1,
		SampleCount:     2,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	require.Equal(t, 1, countProfileRecords(m.profiles))

	p := m.profiles[0]
	dict := p.Dictionary()
	profile := p.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0)
	st := dict.StringTable()
	attrTable := dict.AttributeTable()

	// Profile should have attribute indices
	assert.GreaterOrEqual(t, profile.AttributeIndices().Len(), 2,
		"should have at least 2 attribute indices for k1 and k2")

	// Check that our telemetry attributes are in the attribute table
	foundKeys := map[string]string{}
	for i := 0; i < attrTable.Len(); i++ {
		kvu := attrTable.At(i)
		key := st.At(int(kvu.KeyStrindex()))
		if key == "k1" || key == "k2" {
			foundKeys[key] = kvu.Value().Str()
		}
	}
	assert.Equal(t, "v1", foundKeys["k1"], "attribute k1 should have value v1")
	assert.Equal(t, "v2", foundKeys["k2"], "attribute k2 should have value v2")
}

func TestProfilesPreserveTypedResourceAttributes(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
			ResourceAttributes: config.KeyValue{
				"resource.bool":       true,
				"resource.int":        7,
				"resource.str.slice":  []string{"a", "b"},
				"resource.bool.slice": []bool{true, false},
				"resource.int.slice":  []int{1, 2},
			},
		},
		NumProfiles:     1,
		SampleCount:     2,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	require.NoError(t, run(cfg, exporterFactory, zap.NewNop()))
	require.Equal(t, 1, countProfileRecords(m.profiles))

	attrs := m.profiles[0].ResourceProfiles().At(0).Resource().Attributes()

	boolValue, ok := attrs.Get("resource.bool")
	require.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeBool, boolValue.Type())
	assert.True(t, boolValue.Bool())

	intValue, ok := attrs.Get("resource.int")
	require.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeInt, intValue.Type())
	assert.Equal(t, int64(7), intValue.Int())

	stringSliceValue, ok := attrs.Get("resource.str.slice")
	require.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeSlice, stringSliceValue.Type())
	assert.Equal(t, "a", stringSliceValue.Slice().At(0).Str())
	assert.Equal(t, "b", stringSliceValue.Slice().At(1).Str())

	boolSliceValue, ok := attrs.Get("resource.bool.slice")
	require.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeSlice, boolSliceValue.Type())
	assert.True(t, boolSliceValue.Slice().At(0).Bool())
	assert.False(t, boolSliceValue.Slice().At(1).Bool())

	intSliceValue, ok := attrs.Get("resource.int.slice")
	require.True(t, ok)
	assert.Equal(t, pcommon.ValueTypeSlice, intSliceValue.Type())
	assert.Equal(t, int64(1), intSliceValue.Slice().At(0).Int())
	assert.Equal(t, int64(2), intSliceValue.Slice().At(1).Int())
}

func TestProfilesPreserveTypedTelemetryAttributes(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
			TelemetryAttributes: config.KeyValue{
				"telemetry.bool":       true,
				"telemetry.int":        7,
				"telemetry.str.slice":  []string{"a", "b"},
				"telemetry.bool.slice": []bool{true, false},
				"telemetry.int.slice":  []int{1, 2},
			},
		},
		NumProfiles:     1,
		SampleCount:     2,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	require.NoError(t, run(cfg, exporterFactory, zap.NewNop()))
	require.Equal(t, 1, countProfileRecords(m.profiles))

	dict := m.profiles[0].Dictionary()

	boolValue := findProfileAttribute(t, dict, "telemetry.bool")
	assert.Equal(t, pcommon.ValueTypeBool, boolValue.Type())
	assert.True(t, boolValue.Bool())

	intValue := findProfileAttribute(t, dict, "telemetry.int")
	assert.Equal(t, pcommon.ValueTypeInt, intValue.Type())
	assert.Equal(t, int64(7), intValue.Int())

	stringSliceValue := findProfileAttribute(t, dict, "telemetry.str.slice")
	assert.Equal(t, pcommon.ValueTypeSlice, stringSliceValue.Type())
	assert.Equal(t, "a", stringSliceValue.Slice().At(0).Str())
	assert.Equal(t, "b", stringSliceValue.Slice().At(1).Str())

	boolSliceValue := findProfileAttribute(t, dict, "telemetry.bool.slice")
	assert.Equal(t, pcommon.ValueTypeSlice, boolSliceValue.Type())
	assert.True(t, boolSliceValue.Slice().At(0).Bool())
	assert.False(t, boolSliceValue.Slice().At(1).Bool())

	intSliceValue := findProfileAttribute(t, dict, "telemetry.int.slice")
	assert.Equal(t, pcommon.ValueTypeSlice, intSliceValue.Type())
	assert.Equal(t, int64(1), intSliceValue.Slice().At(0).Int())
	assert.Equal(t, int64(2), intSliceValue.Slice().At(1).Int())
}

func TestBatching(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
			Batch:       true,
			BatchSize:   3,
		},
		NumProfiles:     5,
		SampleCount:     2,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	totalProfiles := countProfileRecords(m.profiles)
	assert.Equal(t, 5, totalProfiles, "should have received all 5 profiles")

	// Should have 2 export calls:
	// First call: 3 profiles (batch size reached)
	// Second call: 2 profiles (remaining flushed at end)
	assert.Len(t, m.exportCalls, 2, "should have made 2 export calls")
}

func TestFlushBufferAllowFailuresContinues(t *testing.T) {
	core, observedLogs := observer.New(zap.DebugLevel)
	exporter := &errorProfileExporter{err: errors.New("export failed")}
	td := pprofile.NewProfiles()
	td.ResourceProfiles().AppendEmpty()
	w := &worker{
		logger:        zap.New(core),
		batchBuffer:   &td,
		bufferCount:   1,
		allowFailures: true,
	}

	w.flushBuffer(exporter)

	require.Equal(t, 1, exporter.exportCalls)
	assert.Equal(t, 0, w.bufferCount)
	require.NotNil(t, w.batchBuffer)
	assert.Equal(t, 0, w.batchBuffer.ResourceProfiles().Len())
	logEntries := observedLogs.All()
	require.Len(t, logEntries, 1)
	assert.Equal(t, zap.ErrorLevel, logEntries[0].Level)
	assert.Contains(t, logEntries[0].Message, "continuing due to --allow-export-failures")
}

func TestFlushBufferDisallowFailuresFatals(t *testing.T) {
	core, observedLogs := observer.New(zap.DebugLevel)
	exporter := &errorProfileExporter{err: errors.New("export failed")}
	td := pprofile.NewProfiles()
	td.ResourceProfiles().AppendEmpty()
	w := &worker{
		logger:      zap.New(core, zap.WithFatalHook(zapcore.WriteThenPanic)),
		batchBuffer: &td,
		bufferCount: 1,
	}

	require.Panics(t, func() {
		w.flushBuffer(exporter)
	})
	require.Equal(t, 1, exporter.exportCalls)
	assert.Equal(t, 1, w.bufferCount)
	assert.Same(t, &td, w.batchBuffer)
	logEntries := observedLogs.All()
	require.Len(t, logEntries, 1)
	assert.Equal(t, zap.FatalLevel, logEntries[0].Level)
	assert.Equal(t, "failed to export batched profiles", logEntries[0].Message)
}

func TestNoBatching(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
			Batch:       false,
		},
		NumProfiles:     5,
		SampleCount:     2,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	totalProfiles := countProfileRecords(m.profiles)
	assert.Equal(t, 5, totalProfiles, "should have received all 5 profiles")

	// Each profile exported individually: 5 export calls
	assert.Len(t, m.exportCalls, 5, "should have made 5 export calls (one per profile)")
	for i, call := range m.exportCalls {
		assert.Equal(t, 1, countProfileRecords([]pprofile.Profiles{call}),
			"export call %d should have exactly 1 profile", i)
	}
}

func TestProfilesWithLoadSize(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
			LoadSize:    2,
		},
		NumProfiles:     1,
		SampleCount:     2,
		StackDepth:      3,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	require.Equal(t, 1, countProfileRecords(m.profiles))

	p := m.profiles[0]
	dict := p.Dictionary()
	profile := p.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0)
	st := dict.StringTable()
	attrTable := dict.AttributeTable()

	// Should have load attributes in the profile's attribute indices
	var load0Found, load1Found bool
	for idx := 0; idx < profile.AttributeIndices().Len(); idx++ {
		attrIdx := profile.AttributeIndices().At(idx)
		kvu := attrTable.At(int(attrIdx))
		key := st.At(int(kvu.KeyStrindex()))
		if key == "load-0" {
			load0Found = true
			assert.Len(t, kvu.Value().Str(), config.CharactersPerMB, "load-0 should have 1MB of data")
		}
		if key == "load-1" {
			load1Found = true
			assert.Len(t, kvu.Value().Str(), config.CharactersPerMB, "load-1 should have 1MB of data")
		}
	}

	assert.True(t, load0Found, "should have load-0 attribute")
	assert.True(t, load1Found, "should have load-1 attribute")
}

func TestBatchingDictionaryIntegrity(t *testing.T) {
	// The logic for creating the dictionaries for profiles is involved,
	// especially when batching is enabled - make sure we're doing it right
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
			Batch:       true,
			BatchSize:   3,
		},
		NumProfiles:     3,
		SampleCount:     2,
		StackDepth:      5,
		UniqueFunctions: 10,
		ProfileDuration: 10 * time.Second,
	}

	m := &mockProfileExporter{}
	exporterFactory := func() (profileExporter, error) {
		return m, nil
	}

	logger := zaptest.NewLogger(t)
	require.NoError(t, run(cfg, exporterFactory, logger))

	require.Len(t, m.exportCalls, 1, "should have exactly 1 export call for a full batch of 3")

	p := m.exportCalls[0]
	assert.Equal(t, 1, p.ResourceProfiles().Len(), "batch should have one ResourceProfiles")
	dict := p.Dictionary()
	st := dict.StringTable()

	// Tables carry 10 real entries plus the reserved zero-value entry at index 0.
	assert.Positive(t, st.Len(), "StringTable should not be empty")
	assert.Equal(t, 11, dict.FunctionTable().Len(), "FunctionTable should have 10 functions plus the zero-value entry")
	assert.Equal(t, 11, dict.LocationTable().Len(), "LocationTable should have one location per function plus the zero-value entry")
	assert.Positive(t, dict.StackTable().Len(), "StackTable should not be empty")

	// Index 0 is the zero-value function; real functions start at index 1.
	fn := dict.FunctionTable().At(1)
	funcName := st.At(int(fn.NameStrindex()))
	assert.True(t, strings.HasPrefix(funcName, "func_"),
		"function name should start with 'func_', got %q", funcName)

	rps := p.ResourceProfiles()
	totalProfiles := 0
	for i := 0; i < rps.Len(); i++ {
		sps := rps.At(i).ScopeProfiles()
		for j := 0; j < sps.Len(); j++ {
			profiles := sps.At(j).Profiles()
			for k := 0; k < profiles.Len(); k++ {
				profile := profiles.At(k)
				for s := 0; s < profile.Samples().Len(); s++ {
					sample := profile.Samples().At(s)
					assert.Less(t, int(sample.StackIndex()), dict.StackTable().Len(),
						"sample stack index %d should be within StackTable bounds (len=%d)",
						sample.StackIndex(), dict.StackTable().Len())
				}
				totalProfiles++
			}
		}
	}
	assert.Equal(t, 3, totalProfiles, "batch should contain all 3 profile records")
}
