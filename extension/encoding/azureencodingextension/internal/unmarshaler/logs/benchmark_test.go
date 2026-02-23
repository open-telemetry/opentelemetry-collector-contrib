// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

func BenchmarkUnmarshalLogs(b *testing.B) {
	tests := map[string]struct {
		nRecords int
	}{
		"1_record": {
			nRecords: 1,
		},
		"100_record": {
			nRecords: 100,
		},
		"1000_record": {
			nRecords: 1_000,
		},
	}

	u := &ResourceLogsUnmarshaler{
		buildInfo: component.BuildInfo{
			Version: "test",
		},
		logger: zap.NewNop(),
	}

	for name, test := range tests {
		buf := newBuf(test.nRecords)
		b.Run(name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				_, err := u.UnmarshalLogs(buf)
				require.NoError(b, err)
			}
		})
	}
}

func BenchmarkUnmarshalLogsByCategory(b *testing.B) {
	testCategoriesDirs, err := os.ReadDir(testFilesDirectory)
	require.NoError(b, err)
	rootPath, err := gojson.CreatePath(unmarshaler.JSONPathEventHubLogRecords)
	require.NoError(b, err)
	u := &ResourceLogsUnmarshaler{
		buildInfo: component.BuildInfo{
			Version: "test",
		},
		logger: zap.NewNop(),
	}

	for _, dir := range testCategoriesDirs {
		if !dir.IsDir() {
			continue
		}

		category := dir.Name()

		testFiles, err := filepath.Glob(filepath.Join(testFilesDirectory, category, "*.json"))
		require.NoError(b, err)

		for _, n := range []int{1, 100, 1_000} {
			b.Run(category+"_records_"+fmt.Sprintf("%d", n), func(b *testing.B) {
				data := getTestRecords(b, rootPath, testFiles, n)
				b.ResetTimer()
				b.ReportAllocs()
				for b.Loop() {
					_, err := u.UnmarshalLogs(data)
					require.NoError(b, err)
				}
			})
		}
	}
}

// newBuf creates an azure resource log with
// as many records as defined in nRecords
func newBuf(nRecords int) []byte {
	data := []byte(`{
		"time": "2024-04-24T12:06:12.0000000Z",
		"resourceId": "/test",
		"category": "AppServiceAppLogs",
		"operationName": "AppLog"
	}`)

	buf := bytes.NewBuffer(make([]byte, 0, nRecords*(len(data))))
	buf.WriteString(`{"records": [`)
	for i := range nRecords {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(data)
	}
	buf.WriteString(`]}`)

	return buf.Bytes()
}

// getTestRecords generates set of test records
// from available testdata files
// as many copies as defined in "n"
func getTestRecords(b *testing.B, rootPath *gojson.Path, files []string, n int) []byte {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(`{"records": [`)
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(b, err)

		records, err := rootPath.Extract(data)
		require.NoError(b, err)
		for _, record := range records {
			for range n {
				buf.Write(record)
				buf.WriteString(",")
			}
		}
	}
	buf.Truncate(buf.Len() - 1)
	buf.WriteString(`]}`)

	return buf.Bytes()
}
