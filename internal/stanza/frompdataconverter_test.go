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

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

func BenchmarkConvertFromPdataSimple(b *testing.B) {
	b.StopTimer()
	pLogs := pdata.NewLogs()
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

func baseMap() pdata.Map {
	obj := pdata.NewMap()
	obj.InsertBool("bool", true)
	obj.InsertInt("int", 123)
	obj.InsertDouble("double", 12.34)
	obj.InsertString("string", "hello")
	obj.InsertBytes("bytes", []byte("asdf"))
	return obj
}

func baseMapValue() pdata.Value {
	v := pdata.NewValueMap()
	baseMap := baseMap()
	baseMap.CopyTo(v.MapVal())
	return v
}

func complexPdataForNDifferentHosts(count int, n int) pdata.Logs {
	pLogs := pdata.NewLogs()
	logs := pLogs.ResourceLogs()

	for i := 0; i < count; i++ {
		rls := logs.AppendEmpty()

		resource := rls.Resource()
		attr := baseMap()
		attr.Insert("object", baseMapValue())
		attr.InsertString("host", fmt.Sprintf("host-%d", i%n))
		attr.CopyTo(resource.Attributes())

		ills := rls.ScopeLogs().AppendEmpty()
		lr := ills.LogRecords().AppendEmpty()
		lr.SetSeverityNumber(pdata.SeverityNumberERROR)
		lr.SetSeverityText("Error")

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

func TestConvertFrom(t *testing.T) {
	entries := ConvertFrom(complexPdataForNDifferentHosts(2, 1))
	require.Equal(t, 2, len(entries))

	for _, e := range entries {
		assert.EqualValues(t,
			map[string]interface{}{
				"host":   "host-0",
				"bool":   true,
				"int":    int64(123),
				"double": 12.34,
				"string": "hello",
				"bytes":  []byte("asdf"),
				"object": map[string]interface{}{
					"bool":   true,
					"int":    int64(123),
					"double": 12.34,
					"string": "hello",
					"bytes":  []byte("asdf"),
				},
			},
			e.Resource,
		)

		assert.EqualValues(t,
			map[string]interface{}{
				"bool":   true,
				"int":    int64(123),
				"string": "hello",
				"bytes":  []byte("asdf"),
				"object": map[string]interface{}{
					"bool":   true,
					"int":    int64(123),
					"double": 12.34,
					"string": "hello",
					"bytes":  []byte("asdf"),
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
				"bytes":  []byte("asdf"),
				"object": map[string]interface{}{
					"bool":   true,
					"int":    int64(123),
					"double": 12.34,
					"string": "hello",
					"bytes":  []byte("asdf"),
					"object": map[string]interface{}{
						"bool":   true,
						"int":    int64(123),
						"double": 12.34,
						"string": "hello",
					},
				},
			},
			e.Body,
		)

		assert.Equal(t, entry.Error, e.Severity)
	}
}

func TestConvertFromSeverity(t *testing.T) {
	cases := []struct {
		expectedSeverity entry.Severity
		severityNumber   pdata.SeverityNumber
	}{
		{entry.Default, pdata.SeverityNumberUNDEFINED},
		{entry.Trace, pdata.SeverityNumberTRACE},
		{entry.Trace2, pdata.SeverityNumberTRACE2},
		{entry.Trace3, pdata.SeverityNumberTRACE3},
		{entry.Trace4, pdata.SeverityNumberTRACE4},
		{entry.Debug, pdata.SeverityNumberDEBUG},
		{entry.Debug2, pdata.SeverityNumberDEBUG2},
		{entry.Debug3, pdata.SeverityNumberDEBUG3},
		{entry.Debug4, pdata.SeverityNumberDEBUG4},
		{entry.Info, pdata.SeverityNumberINFO},
		{entry.Info2, pdata.SeverityNumberINFO2},
		{entry.Info3, pdata.SeverityNumberINFO3},
		{entry.Info4, pdata.SeverityNumberINFO4},
		{entry.Warn, pdata.SeverityNumberWARN},
		{entry.Warn2, pdata.SeverityNumberWARN2},
		{entry.Warn3, pdata.SeverityNumberWARN3},
		{entry.Warn4, pdata.SeverityNumberWARN4},
		{entry.Error, pdata.SeverityNumberERROR},
		{entry.Error2, pdata.SeverityNumberERROR2},
		{entry.Error3, pdata.SeverityNumberERROR3},
		{entry.Error4, pdata.SeverityNumberERROR4},
		{entry.Fatal, pdata.SeverityNumberFATAL},
		{entry.Fatal2, pdata.SeverityNumberFATAL2},
		{entry.Fatal3, pdata.SeverityNumberFATAL3},
		{entry.Fatal4, pdata.SeverityNumberFATAL4},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc.severityNumber), func(t *testing.T) {
			entry := entry.New()
			logRecord := pdata.NewLogRecord()
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
