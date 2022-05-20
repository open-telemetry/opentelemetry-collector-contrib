// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stanza

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func BenchmarkConvertFromPdataSimple(b *testing.B) {
	b.StopTimer()
	pLogs := plog.NewLogs()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		ConvertFrom(pLogs)
	}
}

func BenchmarkConvertFromPdataComplex(b *testing.B) {
	b.StopTimer()
	pLogs := complexPdataForNDifferentHosts(1, 1)
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		ConvertFrom(pLogs)
	}
}

func baseMap() pcommon.Map {
	obj := pcommon.NewMap()
	arr := pcommon.NewValueSlice()
	arr.SliceVal().AppendEmpty().SetStringVal("666")
	arr.SliceVal().AppendEmpty().SetStringVal("777")
	obj.Insert("slice", arr)
	obj.InsertBool("bool", true)
	obj.InsertInt("int", 123)
	obj.InsertDouble("double", 12.34)
	obj.InsertString("string", "hello")
	obj.InsertMBytes("bytes", []byte{0xa1, 0xf0, 0x02, 0xff})
	return obj
}

func baseMapValue() pcommon.Value {
	v := pcommon.NewValueMap()
	baseMap := baseMap()
	baseMap.CopyTo(v.MapVal())
	return v
}

func complexPdataForNDifferentHosts(count int, n int) plog.Logs {
	pLogs := plog.NewLogs()
	logs := pLogs.ResourceLogs()

	for i := 0; i < count; i++ {
		rls := logs.AppendEmpty()

		resource := rls.Resource()
		attr := baseMap()
		attr.Insert("object", baseMapValue())
		attr.InsertString("host", fmt.Sprintf("host-%d", i%n))
		attr.CopyTo(resource.Attributes())

		scopeLog := rls.ScopeLogs().AppendEmpty()
		scopeLog.Scope().SetName("myScope")
		lr := scopeLog.LogRecords().AppendEmpty()

		lr.SetSpanID(pcommon.NewSpanID([8]byte{0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff}))
		lr.SetTraceID(pcommon.NewTraceID([16]byte{0x48, 0x01, 0x40, 0xf3, 0xd7, 0x70, 0xa5, 0xae, 0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff}))
		lr.SetFlags(uint32(0x01))

		lr.SetSeverityNumber(plog.SeverityNumberERROR)
		lr.SetSeverityText("Error")

		t, _ := time.ParseInLocation("2006-01-02", "2022-01-01", time.Local)
		lr.SetTimestamp(pcommon.NewTimestampFromTime(t))
		lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(t))

		attr.Remove("double")
		attr.Remove("host")
		attr.CopyTo(lr.Attributes())

		body := baseMapValue()
		level2 := baseMapValue()
		level2.MapVal().Remove("bytes")
		level1 := baseMapValue()
		level1.MapVal().Insert("object", level2)
		body.MapVal().Insert("object", level1)
		body.CopyTo(lr.Body())
	}
	return pLogs
}

func TestRoundTrip(t *testing.T) {
	initialLogs := complexPdataForNDifferentHosts(1, 1)
	// Converter does not properly aggregate by Scope, until
	// it does so the Round Trip cannot expect it
	initialLogs.ResourceLogs().At(0).ScopeLogs().At(0).Scope().SetName("")
	entries := ConvertFrom(initialLogs)
	require.Equal(t, 1, len(entries))

	pLogs := Convert(entries[0])
	sortComplexData(initialLogs)
	sortComplexData(pLogs)
	require.Equal(t, initialLogs, pLogs)
}

func sortComplexData(pLogs plog.Logs) {
	pLogs.ResourceLogs().At(0).Resource().Attributes().Sort()
	attrObject, _ := pLogs.ResourceLogs().At(0).Resource().Attributes().Get("object")
	attrObject.MapVal().Sort()
	pLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().MapVal().Sort()
	level1, _ := pLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().MapVal().Get("object")
	level1.MapVal().Sort()
	level2, _ := level1.MapVal().Get("object")
	level2.MapVal().Sort()
	pLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Sort()
	attrObject, _ = pLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get("object")
	attrObject.MapVal().Sort()
}

func TestConvertFrom(t *testing.T) {
	entries := ConvertFrom(complexPdataForNDifferentHosts(2, 1))
	require.Equal(t, 2, len(entries))

	for _, e := range entries {
		assert.Equal(t, e.ScopeName, "myScope")
		assert.EqualValues(t,
			map[string]interface{}{
				"host":   "host-0",
				"bool":   true,
				"int":    int64(123),
				"double": 12.34,
				"string": "hello",
				"bytes":  []byte{0xa1, 0xf0, 0x02, 0xff},
				"slice":  []interface{}{"666", "777"},
				"object": map[string]interface{}{
					"bool":   true,
					"int":    int64(123),
					"double": 12.34,
					"string": "hello",
					"slice":  []interface{}{"666", "777"},
					"bytes":  []byte{0xa1, 0xf0, 0x02, 0xff},
				},
			},
			e.Resource,
		)

		assert.EqualValues(t,
			map[string]interface{}{
				"bool":   true,
				"int":    int64(123),
				"string": "hello",
				"slice":  []interface{}{"666", "777"},
				"bytes":  []byte{0xa1, 0xf0, 0x02, 0xff},
				"object": map[string]interface{}{
					"bool":   true,
					"int":    int64(123),
					"double": 12.34,
					"string": "hello",
					"slice":  []interface{}{"666", "777"},
					"bytes":  []byte{0xa1, 0xf0, 0x02, 0xff},
				},
			},
			e.Attributes,
		)

		assert.EqualValues(t,
			map[string]interface{}{
				"bool":   true,
				"int":    int64(123),
				"double": 12.34,
				"string": "hello",
				"slice":  []interface{}{"666", "777"},
				"bytes":  []byte{0xa1, 0xf0, 0x02, 0xff},
				"object": map[string]interface{}{
					"bool":   true,
					"int":    int64(123),
					"double": 12.34,
					"string": "hello",
					"slice":  []interface{}{"666", "777"},
					"bytes":  []byte{0xa1, 0xf0, 0x02, 0xff},
					"object": map[string]interface{}{
						"bool":   true,
						"int":    int64(123),
						"double": 12.34,
						"string": "hello",
						"slice":  []interface{}{"666", "777"},
					},
				},
			},
			e.Body,
		)

		assert.Equal(t, entry.Error, e.Severity)
		assert.Equal(t, []byte{0x48, 0x01, 0x40, 0xf3, 0xd7, 0x70, 0xa5, 0xae, 0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff}, e.TraceId)
		assert.Equal(t, []byte{0x32, 0xf0, 0xa2, 0x2b, 0x6a, 0x81, 0x2c, 0xff}, e.SpanId)
		assert.Equal(t, uint8(0x01), e.TraceFlags[0])
	}
}

func TestConvertFromSeverity(t *testing.T) {
	cases := []struct {
		expectedSeverity entry.Severity
		severityNumber   plog.SeverityNumber
	}{
		{entry.Default, plog.SeverityNumberUNDEFINED},
		{entry.Trace, plog.SeverityNumberTRACE},
		{entry.Trace2, plog.SeverityNumberTRACE2},
		{entry.Trace3, plog.SeverityNumberTRACE3},
		{entry.Trace4, plog.SeverityNumberTRACE4},
		{entry.Debug, plog.SeverityNumberDEBUG},
		{entry.Debug2, plog.SeverityNumberDEBUG2},
		{entry.Debug3, plog.SeverityNumberDEBUG3},
		{entry.Debug4, plog.SeverityNumberDEBUG4},
		{entry.Info, plog.SeverityNumberINFO},
		{entry.Info2, plog.SeverityNumberINFO2},
		{entry.Info3, plog.SeverityNumberINFO3},
		{entry.Info4, plog.SeverityNumberINFO4},
		{entry.Warn, plog.SeverityNumberWARN},
		{entry.Warn2, plog.SeverityNumberWARN2},
		{entry.Warn3, plog.SeverityNumberWARN3},
		{entry.Warn4, plog.SeverityNumberWARN4},
		{entry.Error, plog.SeverityNumberERROR},
		{entry.Error2, plog.SeverityNumberERROR2},
		{entry.Error3, plog.SeverityNumberERROR3},
		{entry.Error4, plog.SeverityNumberERROR4},
		{entry.Fatal, plog.SeverityNumberFATAL},
		{entry.Fatal2, plog.SeverityNumberFATAL2},
		{entry.Fatal3, plog.SeverityNumberFATAL3},
		{entry.Fatal4, plog.SeverityNumberFATAL4},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc.severityNumber), func(t *testing.T) {
			entry := entry.New()
			logRecord := plog.NewLogRecord()
			logRecord.SetSeverityNumber(tc.severityNumber)
			convertFrom(logRecord, entry)
			require.Equal(t, tc.expectedSeverity, entry.Severity)
		})
	}
}

func BenchmarkFromPdataConverter(b *testing.B) {
	const (
		entryCount = 1_000_000
		hostsCount = 4
	)

	var (
		workerCounts = []int{1, 2, 4, 6, 8}
		pLogs        = complexPdataForNDifferentHosts(entryCount, hostsCount)
	)

	for _, wc := range workerCounts {
		b.Run(fmt.Sprintf("worker_count=%d", wc), func(b *testing.B) {
			for i := 0; i < b.N; i++ {

				converter := NewFromPdataConverter(wc, nil)
				converter.Start()
				defer converter.Stop()
				b.ResetTimer()

				go func() {
					assert.NoError(b, converter.Batch(pLogs))
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
					case entries, ok := <-ch:
						if !ok {
							break forLoop
						}

						require.Equal(b, 250_000, len(entries))
						n += len(entries)

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
