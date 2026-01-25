// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlog

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/metadata"
)

func TestType(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop(), component.NewDefaultBuildInfo())
	require.Equal(t, TypeStr, unmarshaler.Type())
}

func TestUnmarshal(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop(), component.NewDefaultBuildInfo())
	testCases := map[string]struct {
		filename               string
		wantResourceCount      int
		wantLogCount           int
		wantErr                error
		wantResourceLogGroups  [][]string
		wantResourceLogStreams [][]string
	}{
		"WithSingleRecord": {
			filename:               "single_record",
			wantResourceCount:      1,
			wantLogCount:           2,
			wantResourceLogGroups:  [][]string{{"test"}},
			wantResourceLogStreams: [][]string{{"test"}},
		},
		"WithInvalidRecords": {
			filename: "invalid_records",
			wantErr:  errInvalidRecords,
		},
		"WithOnlyControlMessages": {
			filename:               "only_control",
			wantResourceCount:      0,
			wantLogCount:           0,
			wantResourceLogGroups:  nil,
			wantResourceLogStreams: nil,
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			record, err := os.ReadFile(filepath.Join(".", "testdata", testCase.filename))
			require.NoError(t, err)

			compressedRecord, err := gzipData(record)
			require.NoError(t, err)

			got, err := unmarshaler.UnmarshalLogs(compressedRecord)
			if testCase.wantErr != nil {
				require.Error(t, err)
				assert.ErrorContains(t, err, testCase.wantErr.Error())
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, testCase.wantResourceCount, got.ResourceLogs().Len())
				gotLogCount := 0
				for i := 0; i < got.ResourceLogs().Len(); i++ {
					rm := got.ResourceLogs().At(i)
					require.Equal(t, 1, rm.ScopeLogs().Len())
					attrs := rm.Resource().Attributes()
					assertString(t, attrs, "cloud.provider", "aws")
					assertString(t, attrs, "cloud.account.id", "123")
					if testCase.wantResourceLogGroups != nil {
						assertStringArray(t, attrs, "aws.log.group.names", testCase.wantResourceLogGroups[i])
					}
					if testCase.wantResourceLogStreams != nil {
						assertStringArray(t, attrs, "aws.log.stream.names", testCase.wantResourceLogStreams[i])
					}
					ilm := rm.ScopeLogs().At(0)
					assert.Equal(t, metadata.ScopeName, ilm.Scope().Name())
					assert.Equal(t, component.NewDefaultBuildInfo().Version, ilm.Scope().Version())
					gotLogCount += ilm.LogRecords().Len()
				}
				require.Equal(t, testCase.wantLogCount, gotLogCount)
			}
		})
	}
}

func TestLogTimestamp(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop(), component.NewDefaultBuildInfo())
	record, err := os.ReadFile(filepath.Join(".", "testdata", "single_record"))
	require.NoError(t, err)

	compressedRecord, err := gzipData(record)
	require.NoError(t, err)

	got, err := unmarshaler.UnmarshalLogs(compressedRecord)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, 1, got.ResourceLogs().Len())

	rm := got.ResourceLogs().At(0)
	require.Equal(t, 1, rm.ScopeLogs().Len())
	ilm := rm.ScopeLogs().At(0)
	ilm.LogRecords().At(0).Timestamp()
	expectedTimestamp := "2024-09-05 13:47:15.523 +0000 UTC"
	require.Equal(t, expectedTimestamp, ilm.LogRecords().At(0).Timestamp().String())
}

func TestUnmarshalLargePayload(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop(), component.NewDefaultBuildInfo())

	var largePayload bytes.Buffer
	largePayload.WriteString(`{"messageType":"DATA_MESSAGE","owner":"123","logGroup":"test","logStream":"test","logEvents":[`)
	largePayload.WriteString(`{"timestamp":1742239784,"message":"`)
	for largePayload.Len() < 5*1024*1024 { // default firehose stream buffer size is 5MB
		largePayload.WriteString("a")
	}
	largePayload.WriteString(`"}]}`)

	compressedRecord, err := gzipData(largePayload.Bytes())
	require.NoError(t, err)

	_, err = unmarshaler.UnmarshalLogs(compressedRecord)
	require.NoError(t, err)
}

func TestUnmarshalExtractedFields_SingleRecord(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop(), component.NewDefaultBuildInfo())
	testCases := map[string]struct {
		filename      string
		wantAccountID string
		wantRegion    string
	}{
		"WithAccountID": {
			filename:      "extracted_fields_account_id",
			wantAccountID: "999888777666",
			wantRegion:    "",
		},
		"WithRegion": {
			filename:      "extracted_fields_region",
			wantAccountID: "123", // Should fall back to Owner
			wantRegion:    "us-west-2",
		},
		"WithBoth": {
			filename:      "extracted_fields_both",
			wantAccountID: "999888777666",
			wantRegion:    "us-west-2",
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			record, err := os.ReadFile(filepath.Join(".", "testdata", testCase.filename))
			require.NoError(t, err)

			compressedRecord, err := gzipData(record)
			require.NoError(t, err)

			got, err := unmarshaler.UnmarshalLogs(compressedRecord)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, 1, got.ResourceLogs().Len())

			rm := got.ResourceLogs().At(0)
			attrs := rm.Resource().Attributes()

			// Verify account ID
			if testCase.wantAccountID != "" {
				assertString(t, attrs, "cloud.account.id", testCase.wantAccountID)
			}

			// Verify region
			if testCase.wantRegion != "" {
				assertString(t, attrs, "cloud.region", testCase.wantRegion)
			} else {
				// Region should not be set if not in extracted fields
				_, ok := attrs.Get("cloud.region")
				assert.False(t, ok, "cloud.region should not be set when not in extractedFields")
			}

			// Verify single log record
			require.Equal(t, 1, rm.ScopeLogs().At(0).LogRecords().Len())
		})
	}
}

func TestUnmarshalExtractedFields_TwoRecords(t *testing.T) {
	unmarshaler := NewUnmarshaler(zap.NewNop(), component.NewDefaultBuildInfo())
	type recordExpectation struct {
		accountID string
		region    string
	}
	testCases := map[string]struct {
		filename          string
		wantResourceCount int
		record1          recordExpectation
		record2          recordExpectation
	}{
		"SameExtractedFields": {
			filename:          "two_records_same_extracted_fields",
			wantResourceCount: 1, // Both records grouped together
			record1: recordExpectation{
				accountID: "999888777666",
				region:    "us-west-2",
			},
			record2: recordExpectation{
				accountID: "999888777666",
				region:    "us-west-2",
			},
		},
		"DifferentExtractedFields": {
			filename:          "two_records_different_extracted_fields",
			wantResourceCount: 2, // Each record gets its own ResourceLogs
			record1: recordExpectation{
				accountID: "111111111111",
				region:    "us-east-1",
			},
			record2: recordExpectation{
				accountID: "222222222222",
				region:    "us-west-2",
			},
		},
		"OneWithExtractedFields": {
			filename:          "two_records_one_with_extracted_fields",
			wantResourceCount: 2, // One with extracted fields, one without
			record1: recordExpectation{
				accountID: "999888777666",
				region:    "us-east-1",
			},
			record2: recordExpectation{
				accountID: "123", // Falls back to Owner
				region:    "",    // No region when no extracted fields
			},
		},
		"NoneWithExtractedFields": {
			filename:          "two_records_none_with_extracted_fields",
			wantResourceCount: 1, // Both use Owner, grouped together
			record1: recordExpectation{
				accountID: "123",
				region:    "",
			},
			record2: recordExpectation{
				accountID: "123",
				region:    "",
			},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			record, err := os.ReadFile(filepath.Join(".", "testdata", testCase.filename))
			require.NoError(t, err)

			compressedRecord, err := gzipData(record)
			require.NoError(t, err)

			got, err := unmarshaler.UnmarshalLogs(compressedRecord)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, testCase.wantResourceCount, got.ResourceLogs().Len())

			// Verify log count
			gotLogCount := 0
			for i := 0; i < got.ResourceLogs().Len(); i++ {
				rm := got.ResourceLogs().At(i)
				gotLogCount += rm.ScopeLogs().At(0).LogRecords().Len()
			}
			require.Equal(t, 2, gotLogCount)

			// Verify each record's resource attributes
			if testCase.wantResourceCount == 1 {
				// Both records in same ResourceLogs
				rm := got.ResourceLogs().At(0)
				attrs := rm.Resource().Attributes()
				assertString(t, attrs, "cloud.account.id", testCase.record1.accountID)
				if testCase.record1.region != "" {
					assertString(t, attrs, "cloud.region", testCase.record1.region)
				} else {
					_, ok := attrs.Get("cloud.region")
					assert.False(t, ok, "cloud.region should not be set when not in extractedFields")
				}
				require.Equal(t, 2, rm.ScopeLogs().At(0).LogRecords().Len())
			} else {
				// Records in separate ResourceLogs - verify each one
				require.Equal(t, 2, got.ResourceLogs().Len())
				
				// Verify record 1
				rm1 := got.ResourceLogs().At(0)
				attrs1 := rm1.Resource().Attributes()
				assertString(t, attrs1, "cloud.account.id", testCase.record1.accountID)
				if testCase.record1.region != "" {
					assertString(t, attrs1, "cloud.region", testCase.record1.region)
				} else {
					_, ok := attrs1.Get("cloud.region")
					assert.False(t, ok, "cloud.region should not be set when not in extractedFields")
				}
				require.Equal(t, 1, rm1.ScopeLogs().At(0).LogRecords().Len())

				// Verify record 2
				rm2 := got.ResourceLogs().At(1)
				attrs2 := rm2.Resource().Attributes()
				assertString(t, attrs2, "cloud.account.id", testCase.record2.accountID)
				if testCase.record2.region != "" {
					assertString(t, attrs2, "cloud.region", testCase.record2.region)
				} else {
					_, ok := attrs2.Get("cloud.region")
					assert.False(t, ok, "cloud.region should not be set when not in extractedFields")
				}
				require.Equal(t, 1, rm2.ScopeLogs().At(0).LogRecords().Len())
			}
		})
	}
}

func gzipData(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)

	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func assertString(t *testing.T, m pcommon.Map, key, expected string) {
	t.Helper()

	v, ok := m.Get(key)
	require.True(t, ok)
	assert.Equal(t, expected, v.AsRaw())
}

func assertStringArray(t *testing.T, m pcommon.Map, key string, expected []string) {
	t.Helper()

	v, ok := m.Get(key)
	require.True(t, ok)
	s := v.Slice().AsRaw()
	vAsStrings := make([]string, len(s))
	for i, v := range s {
		vAsStrings[i] = v.(string)
	}
	assert.ElementsMatch(t, expected, vAsStrings)
}
