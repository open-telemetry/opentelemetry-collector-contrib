// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillogs

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

// createCloudTrailLogContent reads the sample CloudTrail log from testdata
// and duplicates the records to create a log file with the desired number of records.
func createCloudTrailLogContent(b *testing.B, nLogs int) []byte {
	// Read the sample CloudTrail log
	data, err := os.ReadFile("testdata/cloudtrail_logs.json")
	require.NoError(b, err)

	// Parse the sample log to get the records
	var sampleLog CloudTrailLogs
	err = json.Unmarshal(data, &sampleLog)
	require.NoError(b, err)
	require.NotEmpty(b, sampleLog.Records, "sample log should contain at least one record")

	// Create a new log with duplicated records
	benchmarkLog := CloudTrailLogs{
		Records: make([]CloudTrailRecord, 0, nLogs),
	}

	// Duplicate the first record to reach the desired number of logs
	sampleRecord := sampleLog.Records[0]
	for i := 0; i < nLogs; i++ {
		// Create a copy of the record with a slightly modified eventID to ensure uniqueness
		recordCopy := sampleRecord
		recordCopy.EventID = sampleRecord.EventID + "-" + string(rune(i%10+'0'))
		benchmarkLog.Records = append(benchmarkLog.Records, recordCopy)
	}

	// Marshal the log back to JSON
	benchmarkData, err := json.Marshal(benchmarkLog)
	require.NoError(b, err)

	return benchmarkData
}

// compressWithGzip compresses the input data using gzip
func compressWithGzip(b *testing.B, data []byte) []byte {
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)

	_, err := gzipWriter.Write(data)
	require.NoError(b, err)

	err = gzipWriter.Close()
	require.NoError(b, err)

	return buf.Bytes()
}

func BenchmarkUnmarshalLogs(b *testing.B) {
	benchmarks := map[string]struct {
		nLogs    int
		compress bool
	}{
		"1_log": {
			nLogs:    1,
			compress: false,
		},
		"10_logs": {
			nLogs:    10,
			compress: false,
		},
		"100_logs": {
			nLogs:    100,
			compress: false,
		},
		"1000_logs": {
			nLogs:    1000,
			compress: false,
		},
		"1_log_gzipped": {
			nLogs:    1,
			compress: true,
		},
		"100_logs_gzipped": {
			nLogs:    100,
			compress: true,
		},
		"1000_logs_gzipped": {
			nLogs:    1000,
			compress: true,
		},
	}

	u := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})
	for name, benchmark := range benchmarks {
		// Create the log content
		logs := createCloudTrailLogContent(b, benchmark.nLogs)

		// Compress if needed
		if benchmark.compress {
			logs = compressWithGzip(b, logs)
		}

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(logs)))
			for i := 0; i < b.N; i++ {
				_, err := u.UnmarshalLogs(logs)
				if err != nil {
					b.Fatalf("Error unmarshaling logs: %v", err)
				}
			}
		})
	}
}
