// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillogs

import (
	"bytes"
	"compress/gzip"
	"os"
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

// createCloudTrailLogContent reads the sample CloudTrail log from testdata
// and duplicates the records to create a log file with the desired number of records.
// Ensures all records have the same region and account ID.
func createCloudTrailLogContent(b *testing.B, nLogs int) []byte {
	// Read the sample CloudTrail log
	data, err := os.ReadFile("testdata/cloudtrail_logs.json")
	require.NoError(b, err)

	// Parse the sample log to get the records
	var sampleLog CloudTrailLogs
	err = gojson.Unmarshal(data, &sampleLog)
	require.NoError(b, err)
	require.NotEmpty(b, sampleLog.Records, "sample log should contain at least one record")

	// Create a new log with duplicated records
	benchmarkLog := CloudTrailLogs{
		Records: make([]CloudTrailRecord, 0, nLogs),
	}

	// Duplicate the first record to reach the desired number of logs
	sampleRecord := sampleLog.Records[0]
	// Ensure region and account ID are set consistently
	constRegion := "us-east-1"
	constAccountID := "123456789012"
	sampleRecord.AwsRegion = constRegion
	sampleRecord.RecipientAccountID = constAccountID

	for i := range nLogs {
		// Create a copy of the record with a slightly modified eventID to ensure uniqueness
		recordCopy := sampleRecord
		recordCopy.EventID = sampleRecord.EventID + "-" + string(rune(i%10+'0'))
		// Keep region and account ID the same for all records
		recordCopy.AwsRegion = constRegion
		recordCopy.RecipientAccountID = constAccountID
		benchmarkLog.Records = append(benchmarkLog.Records, recordCopy)
	}

	// Marshal the log back to JSON
	benchmarkData, err := gojson.Marshal(benchmarkLog)
	require.NoError(b, err)

	return benchmarkData
}

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
		nLogs int
	}{
		"1_log": {
			nLogs: 1,
		},
		"10_logs": {
			nLogs: 10,
		},
		"100_logs": {
			nLogs: 100,
		},
		"1000_logs": {
			nLogs: 1000,
		},
	}

	u := NewCloudTrailLogsUnmarshaler(component.BuildInfo{})
	for name, benchmark := range benchmarks {
		logs := createCloudTrailLogContent(b, benchmark.nLogs)
		compressedLogs := compressWithGzip(b, logs)

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(compressedLogs)))
			for i := 0; i < b.N; i++ {
				_, err := u.UnmarshalLogs(compressedLogs)
				if err != nil {
					b.Fatalf("Error unmarshaling logs: %v", err)
				}
			}
		})
	}
}
