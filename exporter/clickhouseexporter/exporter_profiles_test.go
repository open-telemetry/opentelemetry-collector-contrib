// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

// buildTestDictionary builds a ProfilesDictionary testing interned strings,
// a mapping, inlined function lines, a multi-location stack, a trace link, and
// both sample and profile level attributes
//
// String table layout (index 0 must be the empty/zero value):
//
//	0 ""  1 "cpu"  2 "nanoseconds"  3 "main"  4 "main.go"  5 "foo"  6 "foo.go"
//	7 "bar"  8 "bar.go"  9 "libc.so"  10 "thread.name"  11 "profile.tag"
func buildTestDictionary(dic pprofile.ProfilesDictionary) {
	dic.StringTable().Append(
		"", "cpu", "nanoseconds", "main", "main.go", "foo", "foo.go",
		"bar", "bar.go", "libc.so", "thread.name", "profile.tag",
	)

	// FunctionTable index 0 is the zero value
	dic.FunctionTable().AppendEmpty()
	fMain := dic.FunctionTable().AppendEmpty() // 1
	fMain.SetNameStrindex(3)
	fMain.SetFilenameStrindex(4)
	fFoo := dic.FunctionTable().AppendEmpty() // 2
	fFoo.SetNameStrindex(5)
	fFoo.SetFilenameStrindex(6)
	fBar := dic.FunctionTable().AppendEmpty() // 3
	fBar.SetNameStrindex(7)
	fBar.SetFilenameStrindex(8)

	// MappingTable index 0 is the zero value
	dic.MappingTable().AppendEmpty()
	mLibc := dic.MappingTable().AppendEmpty() // 1
	mLibc.SetFilenameStrindex(9)

	// LocationTable index 0 is the zero value
	dic.LocationTable().AppendEmpty()
	locFoo := dic.LocationTable().AppendEmpty() // 1 (leaf)
	locFoo.SetMappingIndex(1)
	locFoo.SetAddress(0x1000)
	lineFoo := locFoo.Lines().AppendEmpty()
	lineFoo.SetFunctionIndex(2)
	lineFoo.SetLine(42)

	locCaller := dic.LocationTable().AppendEmpty() // 2 (bar inlined into main)
	locCaller.SetMappingIndex(1)
	locCaller.SetAddress(0x2000)
	// Lines should be leaf-first. The innermost inlined frame comes first, the caller last
	lineBar := locCaller.Lines().AppendEmpty()
	lineBar.SetFunctionIndex(3)
	lineBar.SetLine(10)
	lineMain := locCaller.Lines().AppendEmpty()
	lineMain.SetFunctionIndex(1)
	lineMain.SetLine(99)

	// StackTable: index 0 is the zero value.
	dic.StackTable().AppendEmpty()
	stack := dic.StackTable().AppendEmpty() // 1
	stack.LocationIndices().Append(1, 2)    // leaf-first: foo, then caller(bar->main)

	// LinkTable: index 0 means "no link"
	dic.LinkTable().AppendEmpty()
	link := dic.LinkTable().AppendEmpty() // 1
	link.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	link.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))

	// AttributeTable: index 0 is the zero value.
	dic.AttributeTable().AppendEmpty()
	attrThread := dic.AttributeTable().AppendEmpty() // thread.name=worker-1
	attrThread.SetKeyStrindex(10)
	attrThread.Value().SetStr("worker-1")
	attrTag := dic.AttributeTable().AppendEmpty() // profile.tag=prod
	attrTag.SetKeyStrindex(11)
	attrTag.Value().SetStr("prod")
}

func TestResolveStackFlattensInlinedFramesLeafFirst(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dic := profiles.Dictionary()
	buildTestDictionary(dic)

	rs := resolveStack(dic, 1)

	assert.Equal(t, []string{"foo", "bar", "main"}, rs.functionNames)
	assert.Equal(t, []string{"foo.go", "bar.go", "main.go"}, rs.fileNames)
	assert.Equal(t, []int32{42, 10, 99}, rs.lineNumbers)
	assert.Equal(t, []uint64{0x1000, 0x2000, 0x2000}, rs.addresses)
	assert.Equal(t, []string{"libc.so", "libc.so", "libc.so"}, rs.mappingFileNames)
	assert.NotZero(t, rs.hash)
}

func TestResolveStackIsDeterministicAndDistinct(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dic := profiles.Dictionary()
	buildTestDictionary(dic)

	firstHash := resolveStack(dic, 1).hash
	secondHash := resolveStack(dic, 1).hash
	assert.Equal(t, firstHash, secondHash)
	// The empty stack (index 0) differs from a populated one
	assert.NotEqual(t, resolveStack(dic, 0).hash, resolveStack(dic, 1).hash)
}

func TestResolveStackOutOfRangeIsEmpty(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dic := profiles.Dictionary()
	buildTestDictionary(dic)

	rs := resolveStack(dic, 999)
	assert.Empty(t, rs.functionNames)
	assert.Zero(t, rs.hash)
}

func TestProfileAndSampleAttributesResolveSeparately(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dic := profiles.Dictionary()
	buildTestDictionary(dic)

	profile := pprofile.NewProfile()
	profile.AttributeIndices().Append(2) // profile.tag=prod
	sample := pprofile.NewSample()
	sample.AttributeIndices().Append(1) // thread.name=worker-1

	profileAttrs, err := pprofile.FromAttributeIndices(dic.AttributeTable(), profile, dic)
	require.NoError(t, err)
	sampleAttrs, err := pprofile.FromAttributeIndices(dic.AttributeTable(), sample, dic)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"profile.tag": "prod"}, profileAttrs.AsRaw())
	assert.Equal(t, map[string]any{"thread.name": "worker-1"}, sampleAttrs.AsRaw())
}

func TestResolveLink(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dic := profiles.Dictionary()
	buildTestDictionary(dic)

	traceID, spanID := resolveLink(dic, 1)
	assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", traceID)
	assert.Equal(t, "0102030405060708", spanID)

	// Index 0 means no link
	traceID, spanID = resolveLink(dic, 0)
	assert.Empty(t, traceID)
	assert.Empty(t, spanID)

	// Out-of-range index is treated as no link
	traceID, spanID = resolveLink(dic, 999)
	assert.Empty(t, traceID)
	assert.Empty(t, spanID)
}

func TestStringAt(t *testing.T) {
	profiles := pprofile.NewProfiles()
	dic := profiles.Dictionary()
	buildTestDictionary(dic)
	st := dic.StringTable()

	assert.Equal(t, "cpu", stringAt(st, 1))
	assert.Empty(t, stringAt(st, 0))
	assert.Empty(t, stringAt(st, -1))
	assert.Empty(t, stringAt(st, 9999))
}

func TestRenderProfilesSQL(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultEndpoint

	insertSQL := renderInsertProfilesSQL(cfg)
	assert.Contains(t, insertSQL, `"default"."otel_profiles"`)
	assert.True(t, strings.HasPrefix(insertSQL, "INSERT INTO"))

	createSQL := renderCreateProfilesTableSQL(cfg)
	assert.Contains(t, createSQL, `"default"."otel_profiles"`)
	assert.Contains(t, createSQL, "ENGINE = MergeTree()")
	assert.Contains(t, createSQL, "ORDER BY (ServiceName, SampleType, toDateTime(Timestamp))")

	assert.Contains(t, createSQL, "FunctionNames Array(LowCardinality(String))")
	assert.Contains(t, createSQL, "INDEX idx_function_names FunctionNames TYPE text(tokenizer = 'splitByNonAlpha')")

	assert.Contains(t, createSQL, "`__otel_materialized_k8s.pod.name`")
}

func TestRenderProfilesSQLCustomTableName(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultEndpoint
	cfg.ProfilesTableName = "custom_profiles"

	assert.Contains(t, renderInsertProfilesSQL(cfg), `"default"."custom_profiles"`)
	assert.Contains(t, renderCreateProfilesTableSQL(cfg), `"default"."custom_profiles"`)
}
