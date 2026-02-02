// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

func BenchmarkUnmarshalTraces(b *testing.B) {
	rootPath, err := gojson.CreatePath("$.records[*]")
	require.NoError(b, err)
	u := &ResourceTracesUnmarshaler{
		buildInfo: component.BuildInfo{
			Version: "test",
		},
		logger: zap.NewNop(),
	}

	testFiles, err := filepath.Glob(filepath.Join(testFilesDirectory, "*.json"))
	require.NoError(b, err)

	for _, testFile := range testFiles {
		for _, n := range []int{1, 100, 1_000} {
			testName := strings.TrimSuffix(filepath.Base(testFile), ".json")
			b.Run(testName+"_records_"+fmt.Sprintf("%d", n), func(b *testing.B) {
				data := getTestRecords(b, rootPath, testFiles, n)
				b.ResetTimer()
				b.ReportAllocs()
				for b.Loop() {
					_, err := u.UnmarshalTraces(data)
					require.NoError(b, err)
				}
			})
		}
	}
}

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
