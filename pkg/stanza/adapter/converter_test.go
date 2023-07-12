// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func BenchmarkConvertSimple(b *testing.B) {
	b.StopTimer()
	ent := entry.New()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		convert(ent)
	}
}

func BenchmarkConvertComplex(b *testing.B) {
	b.StopTimer()
	ent := complexEntry()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		convert(ent)
	}
}

func complexEntries(count int) []*entry.Entry {
	return complexEntriesForNDifferentHosts(count, 1)
}

func complexEntriesForNDifferentHosts(count int, n int) []*entry.Entry {
	ret := make([]*entry.Entry, count)
	for i := 0; i < count; i++ {
		e := entry.New()
		e.Severity = entry.Error
		e.Resource = map[string]interface{}{
			"host":   fmt.Sprintf("host-%d", i%n),
			"bool":   true,
			"int":    123,
			"double": 12.34,
			"string": "hello",
			"object": map[string]interface{}{
				"bool":   true,
				"int":    123,
				"double": 12.34,
				"string": "hello",
			},
		}
		e.Body = map[string]interface{}{
			"bool":   true,
			"int":    123,
			"double": 12.34,
			"string": "hello",
			"bytes":  []byte("asdf"),
			"object": map[string]interface{}{
				"bool":   true,
				"int":    123,
				"double": 12.34,
				"string": "hello",
				"bytes":  []byte("asdf"),
				"object": map[string]interface{}{
					"bool":   true,
					"int":    123,
					"double": 12.34,
					"string": "hello",
					"bytes":  []byte("asdf"),
				},
			},
		}
		ret[i] = e
	}
	return ret
}

func complexEntry() *entry.Entry {
	e := entry.New()
	e.Severity = entry.Error
	e.Resource = map[string]interface{}{
		"bool":   true,
		"int":    123,
		"double": 12.34,
		"string": "hello",
		"object": map[string]interface{}{
			"bool":   true,
			"int":    123,
			"double": 12.34,
			"string": "hello",
		},
	}
	e.Attributes = map[string]interface{}{
		"bool":   true,
		"int":    123,
		"double": 12.34,
		"string": "hello",
		"object": map[string]interface{}{
			"bool":   true,
			"int":    123,
			"double": 12.34,
			"string": "hello",
		},
	}
	e.Body = map[string]interface{}{
		"bool":   true,
		"int":    123,
		"double": 12.34,
		"string": "hello",
		// "bytes":  []byte("asdf"),
		"object": map[string]interface{}{
			"bool":   true,
			"int":    123,
			"double": 12.34,
			"string": "hello",
			// "bytes":  []byte("asdf"),
			"object": map[string]interface{}{
				"bool": true,
				"int":  123,
				// "double": 12.34,
				"string": "hello",
				// "bytes":  []byte("asdf"),
			},
		},
	}
	return e
}

func TestHashResource(t *testing.T) {
	testcases := []struct {
		name     string
		baseline map[string]interface{}
		same     []map[string]interface{}
		diff     []map[string]interface{}
	}{
		{
			name:     "empty",
			baseline: map[string]interface{}{},
			same: []map[string]interface{}{
				{},
			},
			diff: []map[string]interface{}{
				{
					"a": "b",
				},
				{
					"a": 1,
				},
			},
		},
		{
			name: "single_string",
			baseline: map[string]interface{}{
				"one": "two",
			},
			same: []map[string]interface{}{
				{
					"one": "two",
				},
			},
			diff: []map[string]interface{}{
				{
					"a": "b",
				},
				{
					"one": 2,
				},
				{
					"one":   "two",
					"three": "four",
				},
			},
		},
		{
			name: "multi_string",
			baseline: map[string]interface{}{
				"one": "two",
				"a":   "b",
			},
			same: []map[string]interface{}{
				{
					"one": "two",
					"a":   "b",
				},
				{
					"a":   "b",
					"one": "two",
				},
			},
			diff: []map[string]interface{}{
				{
					"a": "b",
				},
				{
					"one": "two",
				},
			},
		},
		{
			name: "multi_type",
			baseline: map[string]interface{}{
				"bool":   true,
				"int":    123,
				"double": 12.34,
				"string": "hello",
				"object": map[string]interface{}{},
			},
			same: []map[string]interface{}{
				{
					"bool":   true,
					"int":    123,
					"double": 12.34,
					"string": "hello",
					"object": map[string]interface{}{},
				},
				{
					"object": map[string]interface{}{},
					"double": 12.34,
					"int":    123,
					"bool":   true,
					"string": "hello",
				},
			},
			diff: []map[string]interface{}{
				{
					"bool":   true,
					"int":    123,
					"double": 12.34,
					"string": "hello",
					"object": map[string]interface{}{
						"string": "hello",
					},
				},
			},
		},
		{
			name: "nested",
			baseline: map[string]interface{}{
				"bool":   true,
				"int":    123,
				"double": 12.34,
				"string": "hello",
				"object": map[string]interface{}{
					"bool":   true,
					"int":    123,
					"double": 12.34,
					"string": "hello",
					"object": map[string]interface{}{
						"bool":   true,
						"int":    123,
						"double": 12.34,
						"string": "hello",
						"object": map[string]interface{}{},
					},
				},
			},
			same: []map[string]interface{}{
				{
					"bool":   true,
					"int":    123,
					"double": 12.34,
					"string": "hello",
					"object": map[string]interface{}{
						"bool":   true,
						"int":    123,
						"double": 12.34,
						"string": "hello",
						"object": map[string]interface{}{
							"bool":   true,
							"int":    123,
							"double": 12.34,
							"string": "hello",
							"object": map[string]interface{}{},
						},
					},
				},
			},
			diff: []map[string]interface{}{
				{
					"bool":   true,
					"int":    123,
					"double": 12.34,
					"string": "hello",
					"object": map[string]interface{}{
						"bool":   true,
						"int":    123,
						"double": 12.34,
						"string": "hello",
						"object": map[string]interface{}{},
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			base := HashResource(tc.baseline)
			for _, s := range tc.same {
				require.Equal(t, base, HashResource(s))
			}
			for _, d := range tc.diff {
				require.NotEqual(t, base, HashResource(d))
			}
		})
	}
}

func TestAllConvertedEntriesAreSentAndReceived(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		entries       int
		maxFlushCount uint
	}{
		{
			entries:       10,
			maxFlushCount: 10,
		},
		{
			entries:       10,
			maxFlushCount: 3,
		},
		{
			entries:       100,
			maxFlushCount: 20,
		},
	}

	for i, tc := range testcases {
		tc := tc

		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			converter := NewConverter(zap.NewNop())
			converter.Start()
			defer converter.Stop()

			go func() {
				entries := complexEntries(tc.entries)
				for from := 0; from < tc.entries; from += int(tc.maxFlushCount) {
					to := from + int(tc.maxFlushCount)
					if to > tc.entries {
						to = tc.entries
					}
					assert.NoError(t, converter.Batch(entries[from:to]))
				}
			}()

			var (
				actualCount  int
				timeoutTimer = time.NewTimer(10 * time.Second)
				ch           = converter.OutChannel()
			)
			defer timeoutTimer.Stop()

		forLoop:
			for {
				if tc.entries == actualCount {
					break
				}

				select {
				case pLogs, ok := <-ch:
					if !ok {
						break forLoop
					}

					rLogs := pLogs.ResourceLogs()
					require.Equal(t, 1, rLogs.Len())

					rLog := rLogs.At(0)
					ills := rLog.ScopeLogs()
					require.Equal(t, 1, ills.Len())

					sl := ills.At(0)

					actualCount += sl.LogRecords().Len()

					assert.LessOrEqual(t, uint(sl.LogRecords().Len()), tc.maxFlushCount,
						"Received more log records in one flush than configured by maxFlushCount",
					)

				case <-timeoutTimer.C:
					break forLoop
				}
			}

			assert.Equal(t, tc.entries, actualCount,
				"didn't receive expected number of entries after conversion",
			)
		})
	}
}

func TestConverterCancelledContextCancellsTheFlush(t *testing.T) {
	converter := NewConverter(zap.NewNop())
	converter.Start()
	defer converter.Stop()
	var wg sync.WaitGroup
	wg.Add(1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	go func() {
		defer wg.Done()
		pLogs := plog.NewLogs()
		ills := pLogs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()

		lr := convert(complexEntry())
		lr.CopyTo(ills.LogRecords().AppendEmpty())

		assert.Error(t, converter.flush(ctx, pLogs))
	}()
	wg.Wait()
}

func TestConvertMetadata(t *testing.T) {
	now := time.Now()

	e := entry.New()
	e.Timestamp = now
	e.Severity = entry.Error
	e.AddResourceKey("type", "global")
	e.Attributes = map[string]interface{}{
		"bool":   true,
		"int":    123,
		"double": 12.34,
		"string": "hello",
		"object": map[string]interface{}{
			"bool":   true,
			"int":    123,
			"double": 12.34,
			"string": "hello",
		},
	}
	e.Body = true

	result := convert(e)

	atts := result.Attributes()
	require.Equal(t, 5, atts.Len())

	attVal, ok := atts.Get("bool")
	require.True(t, ok)
	require.True(t, attVal.Bool())

	attVal, ok = atts.Get("int")
	require.True(t, ok)
	require.Equal(t, int64(123), attVal.Int())

	attVal, ok = atts.Get("double")
	require.True(t, ok)
	require.Equal(t, 12.34, attVal.Double())

	attVal, ok = atts.Get("string")
	require.True(t, ok)
	require.Equal(t, "hello", attVal.Str())

	attVal, ok = atts.Get("object")
	require.True(t, ok)

	mapVal := attVal.Map()
	require.Equal(t, 4, mapVal.Len())

	attVal, ok = mapVal.Get("bool")
	require.True(t, ok)
	require.True(t, attVal.Bool())

	attVal, ok = mapVal.Get("int")
	require.True(t, ok)
	require.Equal(t, int64(123), attVal.Int())

	attVal, ok = mapVal.Get("double")
	require.True(t, ok)
	require.Equal(t, 12.34, attVal.Double())

	attVal, ok = mapVal.Get("string")
	require.True(t, ok)
	require.Equal(t, "hello", attVal.Str())

	bod := result.Body()
	require.Equal(t, pcommon.ValueTypeBool, bod.Type())
	require.True(t, bod.Bool())
}

func TestConvertSimpleBody(t *testing.T) {
	require.True(t, anyToBody(true).Bool())
	require.False(t, anyToBody(false).Bool())

	require.Equal(t, "string", anyToBody("string").Str())
	require.Equal(t, []byte("bytes"), anyToBody([]byte("bytes")).Bytes().AsRaw())

	require.Equal(t, int64(1), anyToBody(1).Int())
	require.Equal(t, int64(1), anyToBody(int8(1)).Int())
	require.Equal(t, int64(1), anyToBody(int16(1)).Int())
	require.Equal(t, int64(1), anyToBody(int32(1)).Int())
	require.Equal(t, int64(1), anyToBody(int64(1)).Int())

	require.Equal(t, int64(1), anyToBody(uint(1)).Int())
	require.Equal(t, int64(1), anyToBody(uint8(1)).Int())
	require.Equal(t, int64(1), anyToBody(uint16(1)).Int())
	require.Equal(t, int64(1), anyToBody(uint32(1)).Int())
	require.Equal(t, int64(1), anyToBody(uint64(1)).Int())

	require.Equal(t, float64(1), anyToBody(float32(1)).Double())
	require.Equal(t, float64(1), anyToBody(float64(1)).Double())
}

func TestConvertMapBody(t *testing.T) {
	structuredBody := map[string]interface{}{
		"true":    true,
		"false":   false,
		"string":  "string",
		"bytes":   []byte("bytes"),
		"int":     1,
		"int8":    int8(1),
		"int16":   int16(1),
		"int32":   int32(1),
		"int64":   int64(1),
		"uint":    uint(1),
		"uint8":   uint8(1),
		"uint16":  uint16(1),
		"uint32":  uint32(1),
		"uint64":  uint64(1),
		"float32": float32(1),
		"float64": float64(1),
	}

	result := anyToBody(structuredBody).Map()

	v, _ := result.Get("true")
	require.True(t, v.Bool())
	v, _ = result.Get("false")
	require.False(t, v.Bool())

	for _, k := range []string{"string"} {
		v, _ = result.Get(k)
		require.Equal(t, k, v.Str())
	}
	for _, k := range []string{"bytes"} {
		v, _ = result.Get(k)
		require.Equal(t, []byte(k), v.Bytes().AsRaw())
	}
	for _, k := range []string{"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64"} {
		v, _ = result.Get(k)
		require.Equal(t, int64(1), v.Int())
	}
	for _, k := range []string{"float32", "float64"} {
		v, _ = result.Get(k)
		require.Equal(t, float64(1), v.Double())
	}
}

func TestConvertArrayBody(t *testing.T) {
	structuredBody := []interface{}{
		true,
		false,
		"string",
		[]byte("bytes"),
		1,
		int8(1),
		int16(1),
		int32(1),
		int64(1),
		uint(1),
		uint8(1),
		uint16(1),
		uint32(1),
		uint64(1),
		float32(1),
		float64(1),
		[]interface{}{"string", 1},
		map[string]interface{}{"one": 1, "yes": true},
	}

	result := anyToBody(structuredBody).Slice()

	require.True(t, result.At(0).Bool())
	require.False(t, result.At(1).Bool())
	require.Equal(t, "string", result.At(2).Str())
	require.Equal(t, []byte("bytes"), result.At(3).Bytes().AsRaw())

	require.Equal(t, int64(1), result.At(4).Int())  // int
	require.Equal(t, int64(1), result.At(5).Int())  // int8
	require.Equal(t, int64(1), result.At(6).Int())  // int16
	require.Equal(t, int64(1), result.At(7).Int())  // int32
	require.Equal(t, int64(1), result.At(8).Int())  // int64
	require.Equal(t, int64(1), result.At(9).Int())  // uint
	require.Equal(t, int64(1), result.At(10).Int()) // uint8
	require.Equal(t, int64(1), result.At(11).Int()) // uint16
	require.Equal(t, int64(1), result.At(12).Int()) // uint32
	require.Equal(t, int64(1), result.At(13).Int()) // uint64

	require.Equal(t, float64(1), result.At(14).Double()) // float32
	require.Equal(t, float64(1), result.At(15).Double()) // float64

	nestedArr := result.At(16).Slice()
	require.Equal(t, "string", nestedArr.At(0).Str())
	require.Equal(t, int64(1), nestedArr.At(1).Int())

	nestedMap := result.At(17).Map()
	v, _ := nestedMap.Get("one")
	require.Equal(t, int64(1), v.Int())
	v, _ = nestedMap.Get("yes")
	require.True(t, v.Bool())
}

func TestConvertNilBody(t *testing.T) {
	require.Equal(t, plog.NewLogRecord().Body(), anyToBody(nil))
}

func TestConvertUnknownBody(t *testing.T) {
	unknownType := map[string]int{"0": 0, "1": 1}
	require.Equal(t, fmt.Sprintf("%v", unknownType), anyToBody(unknownType).Str())
}

func TestConvertNestedMapBody(t *testing.T) {
	unknownType := map[string]int{"0": 0, "1": 1}

	structuredBody := map[string]interface{}{
		"array":   []interface{}{0, 1},
		"map":     map[string]interface{}{"0": 0, "1": "one"},
		"unknown": unknownType,
	}

	result := anyToBody(structuredBody).Map()

	arrayAttVal, _ := result.Get("array")
	a := arrayAttVal.Slice()
	require.Equal(t, int64(0), a.At(0).Int())
	require.Equal(t, int64(1), a.At(1).Int())

	mapAttVal, _ := result.Get("map")
	m := mapAttVal.Map()
	v, _ := m.Get("0")
	require.Equal(t, int64(0), v.Int())
	v, _ = m.Get("1")
	require.Equal(t, "one", v.Str())

	unknownAttVal, _ := result.Get("unknown")
	require.Equal(t, fmt.Sprintf("%v", unknownType), unknownAttVal.Str())
}

func anyToBody(body interface{}) pcommon.Value {
	entry := entry.New()
	entry.Body = body
	return convert(entry).Body()
}

func TestConvertSeverity(t *testing.T) {
	cases := []struct {
		severity       entry.Severity
		severityText   string
		expectedNumber plog.SeverityNumber
		expectedText   string
	}{
		{entry.Default, "", plog.SeverityNumberUnspecified, ""},
		{entry.Trace, "Trace", plog.SeverityNumberTrace, "Trace"},
		{entry.Trace2, "Trace2", plog.SeverityNumberTrace2, "Trace2"},
		{entry.Trace3, "Trace3", plog.SeverityNumberTrace3, "Trace3"},
		{entry.Trace4, "Trace4", plog.SeverityNumberTrace4, "Trace4"},
		{entry.Debug, "Debug", plog.SeverityNumberDebug, "Debug"},
		{entry.Debug2, "Debug2", plog.SeverityNumberDebug2, "Debug2"},
		{entry.Debug3, "Debug3", plog.SeverityNumberDebug3, "Debug3"},
		{entry.Debug4, "Debug4", plog.SeverityNumberDebug4, "Debug4"},
		{entry.Info, "Info", plog.SeverityNumberInfo, "Info"},
		{entry.Info2, "Info2", plog.SeverityNumberInfo2, "Info2"},
		{entry.Info3, "Info3", plog.SeverityNumberInfo3, "Info3"},
		{entry.Info4, "Info4", plog.SeverityNumberInfo4, "Info4"},
		{entry.Warn, "Warn", plog.SeverityNumberWarn, "Warn"},
		{entry.Warn2, "Warn2", plog.SeverityNumberWarn2, "Warn2"},
		{entry.Warn3, "Warn3", plog.SeverityNumberWarn3, "Warn3"},
		{entry.Warn4, "Warn4", plog.SeverityNumberWarn4, "Warn4"},
		{entry.Error, "Error", plog.SeverityNumberError, "Error"},
		{entry.Error2, "Error2", plog.SeverityNumberError2, "Error2"},
		{entry.Error3, "Error3", plog.SeverityNumberError3, "Error3"},
		{entry.Error4, "Error4", plog.SeverityNumberError4, "Error4"},
		{entry.Fatal, "Fatal", plog.SeverityNumberFatal, "Fatal"},
		{entry.Fatal2, "Fatal2", plog.SeverityNumberFatal2, "Fatal2"},
		{entry.Fatal3, "Fatal3", plog.SeverityNumberFatal3, "Fatal3"},
		{entry.Fatal4, "Fatal4", plog.SeverityNumberFatal4, "Fatal4"},

		// Original severity text should be preserved if present
		{entry.Trace, "other", plog.SeverityNumberTrace, "other"},
		{entry.Trace2, "other", plog.SeverityNumberTrace2, "other"},
		{entry.Trace3, "other", plog.SeverityNumberTrace3, "other"},
		{entry.Trace4, "other", plog.SeverityNumberTrace4, "other"},
		{entry.Debug, "other", plog.SeverityNumberDebug, "other"},
		{entry.Debug2, "other", plog.SeverityNumberDebug2, "other"},
		{entry.Debug3, "other", plog.SeverityNumberDebug3, "other"},
		{entry.Debug4, "other", plog.SeverityNumberDebug4, "other"},
		{entry.Info, "other", plog.SeverityNumberInfo, "other"},
		{entry.Info2, "other", plog.SeverityNumberInfo2, "other"},
		{entry.Info3, "other", plog.SeverityNumberInfo3, "other"},
		{entry.Info4, "other", plog.SeverityNumberInfo4, "other"},
		{entry.Warn, "other", plog.SeverityNumberWarn, "other"},
		{entry.Warn2, "other", plog.SeverityNumberWarn2, "other"},
		{entry.Warn3, "other", plog.SeverityNumberWarn3, "other"},
		{entry.Warn4, "other", plog.SeverityNumberWarn4, "other"},
		{entry.Error, "other", plog.SeverityNumberError, "other"},
		{entry.Error2, "other", plog.SeverityNumberError2, "other"},
		{entry.Error3, "other", plog.SeverityNumberError3, "other"},
		{entry.Error4, "other", plog.SeverityNumberError4, "other"},
		{entry.Fatal, "other", plog.SeverityNumberFatal, "other"},
		{entry.Fatal2, "other", plog.SeverityNumberFatal2, "other"},
		{entry.Fatal3, "other", plog.SeverityNumberFatal3, "other"},
		{entry.Fatal4, "other", plog.SeverityNumberFatal4, "other"},

		// Sev text should be set to severity "Short Name" if not present
		{entry.Trace, "", plog.SeverityNumberTrace, "TRACE"},
		{entry.Trace2, "", plog.SeverityNumberTrace2, "TRACE2"},
		{entry.Trace3, "", plog.SeverityNumberTrace3, "TRACE3"},
		{entry.Trace4, "", plog.SeverityNumberTrace4, "TRACE4"},
		{entry.Debug, "", plog.SeverityNumberDebug, "DEBUG"},
		{entry.Debug2, "", plog.SeverityNumberDebug2, "DEBUG2"},
		{entry.Debug3, "", plog.SeverityNumberDebug3, "DEBUG3"},
		{entry.Debug4, "", plog.SeverityNumberDebug4, "DEBUG4"},
		{entry.Info, "", plog.SeverityNumberInfo, "INFO"},
		{entry.Info2, "", plog.SeverityNumberInfo2, "INFO2"},
		{entry.Info3, "", plog.SeverityNumberInfo3, "INFO3"},
		{entry.Info4, "", plog.SeverityNumberInfo4, "INFO4"},
		{entry.Warn, "", plog.SeverityNumberWarn, "WARN"},
		{entry.Warn2, "", plog.SeverityNumberWarn2, "WARN2"},
		{entry.Warn3, "", plog.SeverityNumberWarn3, "WARN3"},
		{entry.Warn4, "", plog.SeverityNumberWarn4, "WARN4"},
		{entry.Error, "", plog.SeverityNumberError, "ERROR"},
		{entry.Error2, "", plog.SeverityNumberError2, "ERROR2"},
		{entry.Error3, "", plog.SeverityNumberError3, "ERROR3"},
		{entry.Error4, "", plog.SeverityNumberError4, "ERROR4"},
		{entry.Fatal, "", plog.SeverityNumberFatal, "FATAL"},
		{entry.Fatal2, "", plog.SeverityNumberFatal2, "FATAL2"},
		{entry.Fatal3, "", plog.SeverityNumberFatal3, "FATAL3"},
		{entry.Fatal4, "", plog.SeverityNumberFatal4, "FATAL4"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc.severity), func(t *testing.T) {
			entry := entry.New()
			entry.Severity = tc.severity
			entry.SeverityText = tc.severityText
			log := convert(entry)
			require.Equal(t, tc.expectedNumber, log.SeverityNumber())
			require.Equal(t, tc.expectedText, log.SeverityText())
		})
	}
}

func TestConvertTrace(t *testing.T) {
	record := convert(&entry.Entry{
		TraceID: []byte{
			0x48, 0x01, 0x40, 0xf3, 0xd7, 0x70, 0xa5, 0xae, 0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff,
		},
		SpanID: []byte{
			0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff,
		},
		TraceFlags: []byte{
			0x01,
		}})

	require.Equal(t, pcommon.TraceID(
		[16]byte{
			0x48, 0x01, 0x40, 0xf3, 0xd7, 0x70, 0xa5, 0xae, 0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff,
		}), record.TraceID())
	require.Equal(t, pcommon.SpanID(
		[8]byte{
			0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff,
		}), record.SpanID())
	require.Equal(t, uint32(0x01), uint32(record.Flags()))
}

func TestConvertTraceEmptyFlags(t *testing.T) {
	record := convert(&entry.Entry{
		TraceID: []byte{
			0x48, 0x01, 0x40, 0xf3, 0xd7, 0x70, 0xa5, 0xae, 0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff,
		},
		SpanID: []byte{
			0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff,
		},
		TraceFlags: []byte{}})

	require.Equal(t, pcommon.TraceID(
		[16]byte{
			0x48, 0x01, 0x40, 0xf3, 0xd7, 0x70, 0xa5, 0xae, 0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff,
		}), record.TraceID())
	require.Equal(t, pcommon.SpanID(
		[8]byte{
			0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff,
		}), record.SpanID())
	require.Equal(t, uint32(0x00), uint32(record.Flags()))
}

func BenchmarkConverter(b *testing.B) {
	const (
		entryCount = 1_000_000
		hostsCount = 4
		batchSize  = 200
	)

	var (
		workerCounts = []int{1, 2, 4, 6, 8}
		entries      = complexEntriesForNDifferentHosts(entryCount, hostsCount)
	)

	for _, wc := range workerCounts {
		b.Run(fmt.Sprintf("worker_count=%d", wc), func(b *testing.B) {
			for i := 0; i < b.N; i++ {

				converter := NewConverter(zap.NewNop(), withWorkerCount(wc))
				converter.Start()
				defer converter.Stop()

				b.ReportAllocs()

				go func() {
					for from := 0; from < entryCount; from += int(batchSize) {
						to := from + int(batchSize)
						if to > entryCount {
							to = entryCount
						}
						assert.NoError(b, converter.Batch(entries[from:to]))
					}
				}()

				var (
					timeoutTimer = time.NewTimer(10 * time.Second)
					ch           = converter.OutChannel()
				)
				defer timeoutTimer.Stop()

				var n int
			forLoop:
				for {
					if n == entryCount {
						break
					}

					select {
					case pLogs, ok := <-ch:
						if !ok {
							break forLoop
						}

						rLogs := pLogs.ResourceLogs()
						require.Equal(b, hostsCount, rLogs.Len())
						n += pLogs.LogRecordCount()

					case <-timeoutTimer.C:
						break forLoop
					}
				}

				assert.Equal(b, entryCount, n,
					"didn't receive expected number of entries after conversion",
				)
			}
		})
	}
}

func BenchmarkGetResourceID(b *testing.B) {
	b.StopTimer()
	res := getResource()
	b.ReportAllocs()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		HashResource(res)
	}
}

func BenchmarkGetResourceIDEmptyResource(b *testing.B) {
	res := map[string]interface{}{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashResource(res)
	}
}

func BenchmarkGetResourceIDSingleResource(b *testing.B) {
	res := map[string]interface{}{
		"resource": "value",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashResource(res)
	}
}

func BenchmarkGetResourceIDComplexResource(b *testing.B) {
	res := map[string]interface{}{
		"resource": "value",
		"object": map[string]interface{}{
			"one":   "two",
			"three": 4,
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashResource(res)
	}
}

func getResource() map[string]interface{} {
	return map[string]interface{}{
		"file.name":        "filename.log",
		"file.directory":   "/some_directory",
		"host.name":        "localhost",
		"host.ip":          "192.168.1.12",
		"k8s.pod.name":     "test-pod-123zwe1",
		"k8s.node.name":    "aws-us-east-1.asfasf.aws.com",
		"k8s.container.id": "192end1yu823aocajsiocjnasd",
		"k8s.cluster.name": "my-cluster",
	}
}

type resourceIDOutput struct {
	name   string
	output uint64
}

type resourceIDOutputSlice []resourceIDOutput

func (r resourceIDOutputSlice) Len() int {
	return len(r)
}

func (r resourceIDOutputSlice) Less(i, j int) bool {
	return r[i].output < r[j].output
}

func (r resourceIDOutputSlice) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func TestGetResourceID(t *testing.T) {
	testCases := []struct {
		name  string
		input map[string]interface{}
	}{
		{
			name:  "Typical Resource",
			input: getResource(),
		},
		{
			name: "Empty value/key",
			input: map[string]interface{}{
				"SomeKey": "",
				"":        "Ooops",
			},
		},
		{
			name: "Empty value/key (reversed)",
			input: map[string]interface{}{
				"":      "SomeKey",
				"Ooops": "",
			},
		},
		{
			name: "Ambiguous map 1",
			input: map[string]interface{}{
				"AB": "CD",
				"EF": "G",
			},
		},
		{
			name: "Ambiguous map 2",
			input: map[string]interface{}{
				"ABC": "DE",
				"F":   "G",
			},
		},
		{
			name:  "nil resource",
			input: nil,
		},
		{
			name: "Long resource value",
			input: map[string]interface{}{
				"key": "This is a really long resource value; It's so long that the internal pre-allocated buffer doesn't hold it.",
			},
		},
	}

	outputs := resourceIDOutputSlice{}
	for _, testCase := range testCases {
		outputs = append(outputs, resourceIDOutput{
			name:   testCase.name,
			output: HashResource(testCase.input),
		})
	}

	// Ensure every output is unique
	sort.Sort(outputs)
	for i := 1; i < len(outputs); i++ {
		if outputs[i].output == outputs[i-1].output {
			t.Errorf("Test case %s and %s had the same output", outputs[i].name, outputs[i-1].name)
		}
	}
}

func TestGetResourceIDEmptyAndNilAreEqual(t *testing.T) {
	nilID := HashResource(nil)
	emptyID := HashResource(map[string]interface{}{})
	require.Equal(t, nilID, emptyID)
}
