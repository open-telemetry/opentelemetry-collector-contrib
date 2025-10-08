// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillog

import (
	"bytes"
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
	data, err := os.ReadFile("testdata/cloudtrail_log.json")
	require.NoError(b, err)

	// Parse the sample log to get the records
	var sampleLog CloudTrailLog
	err = gojson.Unmarshal(data, &sampleLog)
	require.NoError(b, err)
	require.NotEmpty(b, sampleLog.Records, "sample log should contain at least one record")

	// Create a new log with duplicated records
	benchmarkLog := CloudTrailLog{
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

	u := NewCloudTrailLogUnmarshaler(component.BuildInfo{})
	for name, benchmark := range benchmarks {
		// Generate the log content with the specified number of records
		logContent := createCloudTrailLogContent(b, benchmark.nLogs)

		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(logContent)))
			b.ResetTimer()

			for b.Loop() {
				// Create a new reader for each iteration to ensure consistent behavior
				reader := bytes.NewReader(logContent)

				// Unmarshal the logs
				_, err := u.UnmarshalAWSLogs(reader)
				if err != nil {
					b.Fatalf("Error unmarshaling logs: %v", err)
				}
			}
		})
	}
}
