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

// func TestConvertFromMapBody(t *testing.T) {
// 	structuredBody := map[string]interface{}{
// 		"true":    true,
// 		"false":   false,
// 		"string":  "string",
// 		"bytes":   []byte("bytes"),
// 		"int":     1,
// 		"int8":    int8(1),
// 		"int16":   int16(1),
// 		"int32":   int32(1),
// 		"int64":   int64(1),
// 		"uint":    uint(1),
// 		"uint8":   uint8(1),
// 		"uint16":  uint16(1),
// 		"uint32":  uint32(1),
// 		"uint64":  uint64(1),
// 		"float32": float32(1),
// 		"float64": float64(1),
// 	}

// 	result := anyToBody(structuredBody).MapVal()

// 	v, _ := result.Get("true")
// 	require.True(t, v.BoolVal())
// 	v, _ = result.Get("false")
// 	require.False(t, v.BoolVal())

// 	for _, k := range []string{"string", "bytes"} {
// 		v, _ = result.Get(k)
// 		require.Equal(t, k, v.StringVal())
// 	}
// 	for _, k := range []string{"int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64"} {
// 		v, _ = result.Get(k)
// 		require.Equal(t, int64(1), v.IntVal())
// 	}
// 	for _, k := range []string{"float32", "float64"} {
// 		v, _ = result.Get(k)
// 		require.Equal(t, float64(1), v.DoubleVal())
// 	}
// }

// func TestConvertFromArrayBody(t *testing.T) {
// 	structuredBody := []interface{}{
// 		true,
// 		false,
// 		"string",
// 		[]byte("bytes"),
// 		1,
// 		int8(1),
// 		int16(1),
// 		int32(1),
// 		int64(1),
// 		uint(1),
// 		uint8(1),
// 		uint16(1),
// 		uint32(1),
// 		uint64(1),
// 		float32(1),
// 		float64(1),
// 		[]interface{}{"string", 1},
// 		map[string]interface{}{"one": 1, "yes": true},
// 	}

// 	result := anyToBody(structuredBody).SliceVal()

// 	require.True(t, result.At(0).BoolVal())
// 	require.False(t, result.At(1).BoolVal())
// 	require.Equal(t, "string", result.At(2).StringVal())
// 	require.Equal(t, "bytes", result.At(3).StringVal())

// 	require.Equal(t, int64(1), result.At(4).IntVal())  // int
// 	require.Equal(t, int64(1), result.At(5).IntVal())  // int8
// 	require.Equal(t, int64(1), result.At(6).IntVal())  // int16
// 	require.Equal(t, int64(1), result.At(7).IntVal())  // int32
// 	require.Equal(t, int64(1), result.At(8).IntVal())  // int64
// 	require.Equal(t, int64(1), result.At(9).IntVal())  // uint
// 	require.Equal(t, int64(1), result.At(10).IntVal()) // uint8
// 	require.Equal(t, int64(1), result.At(11).IntVal()) // uint16
// 	require.Equal(t, int64(1), result.At(12).IntVal()) // uint32
// 	require.Equal(t, int64(1), result.At(13).IntVal()) // uint64

// 	require.Equal(t, float64(1), result.At(14).DoubleVal()) // float32
// 	require.Equal(t, float64(1), result.At(15).DoubleVal()) // float64

// 	nestedArr := result.At(16).SliceVal()
// 	require.Equal(t, "string", nestedArr.At(0).StringVal())
// 	require.Equal(t, int64(1), nestedArr.At(1).IntVal())

// 	nestedMap := result.At(17).MapVal()
// 	v, _ := nestedMap.Get("one")
// 	require.Equal(t, int64(1), v.IntVal())
// 	v, _ = nestedMap.Get("yes")
// 	require.True(t, v.BoolVal())
// }

// func TestConvertFromUnknownBody(t *testing.T) {
// 	unknownType := map[string]int{"0": 0, "1": 1}
// 	require.Equal(t, fmt.Sprintf("%v", unknownType), anyToBody(unknownType).StringVal())
// }

// func TestConvertFromSeverity(t *testing.T) {
// 	cases := []struct {
// 		severity       entry.Severity
// 		expectedNumber pdata.SeverityNumber
// 		expectedText   string
// 	}{
// 		{entry.Default, pdata.SeverityNumberUNDEFINED, ""},
// 		{entry.Trace, pdata.SeverityNumberTRACE, "Trace"},
// 		{entry.Trace2, pdata.SeverityNumberTRACE2, "Trace2"},
// 		{entry.Trace3, pdata.SeverityNumberTRACE3, "Trace3"},
// 		{entry.Trace4, pdata.SeverityNumberTRACE4, "Trace4"},
// 		{entry.Debug, pdata.SeverityNumberDEBUG, "Debug"},
// 		{entry.Debug2, pdata.SeverityNumberDEBUG2, "Debug2"},
// 		{entry.Debug3, pdata.SeverityNumberDEBUG3, "Debug3"},
// 		{entry.Debug4, pdata.SeverityNumberDEBUG4, "Debug4"},
// 		{entry.Info, pdata.SeverityNumberINFO, "Info"},
// 		{entry.Info2, pdata.SeverityNumberINFO2, "Info2"},
// 		{entry.Info3, pdata.SeverityNumberINFO3, "Info3"},
// 		{entry.Info4, pdata.SeverityNumberINFO4, "Info4"},
// 		{entry.Warn, pdata.SeverityNumberWARN, "Warn"},
// 		{entry.Warn2, pdata.SeverityNumberWARN2, "Warn2"},
// 		{entry.Warn3, pdata.SeverityNumberWARN3, "Warn3"},
// 		{entry.Warn4, pdata.SeverityNumberWARN4, "Warn4"},
// 		{entry.Error, pdata.SeverityNumberERROR, "Error"},
// 		{entry.Error2, pdata.SeverityNumberERROR2, "Error2"},
// 		{entry.Error3, pdata.SeverityNumberERROR3, "Error3"},
// 		{entry.Error4, pdata.SeverityNumberERROR4, "Error4"},
// 		{entry.Fatal, pdata.SeverityNumberFATAL, "Fatal"},
// 		{entry.Fatal2, pdata.SeverityNumberFATAL2, "Fatal2"},
// 		{entry.Fatal3, pdata.SeverityNumberFATAL3, "Fatal3"},
// 		{entry.Fatal4, pdata.SeverityNumberFATAL4, "Fatal4"},
// 	}

// 	for _, tc := range cases {
// 		t.Run(fmt.Sprintf("%v", tc.severity), func(t *testing.T) {
// 			entry := entry.New()
// 			entry.Severity = tc.severity
// 			log := convertAndDrill(entry)
// 			require.Equal(t, tc.expectedNumber, log.SeverityNumber())
// 			require.Equal(t, tc.expectedText, log.SeverityText())
// 		})
// 	}
// }

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
