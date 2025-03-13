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
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
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
		"WithMultipleRecords": {
			filename:               "multiple_records",
			wantResourceCount:      1,
			wantLogCount:           2,
			wantResourceLogGroups:  [][]string{{"test"}},
			wantResourceLogStreams: [][]string{{"test"}},
		},
		"WithSingleRecord": {
			filename:               "single_record",
			wantResourceCount:      1,
			wantLogCount:           1,
			wantResourceLogGroups:  [][]string{{"test"}},
			wantResourceLogStreams: [][]string{{"test"}},
		},
		"WithInvalidRecords": {
			filename: "invalid_records",
			wantErr:  errInvalidRecords,
		},
		"WithSomeInvalidRecords": {
			filename:               "some_invalid_records",
			wantResourceCount:      1,
			wantLogCount:           2,
			wantResourceLogGroups:  [][]string{{"test"}},
			wantResourceLogStreams: [][]string{{"test"}},
		},
		"WithMultipleResources": {
			filename:               "multiple_resources",
			wantResourceCount:      3,
			wantLogCount:           6,
			wantResourceLogGroups:  nil, // not checking log group names because logs are unordered
			wantResourceLogStreams: nil, // not checking log stream names because logs are unordered
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
				require.Equal(t, testCase.wantErr, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, testCase.wantResourceCount, got.ResourceLogs().Len())
				gotLogCount := 0
				for i := 0; i < got.ResourceLogs().Len(); i++ {
					rm := got.ResourceLogs().At(i)
					require.Equal(t, 1, rm.ScopeLogs().Len())
					attrs := rm.Resource().Attributes()
					assertString(t, attrs, conventions.AttributeCloudProvider, "aws")
					assertString(t, attrs, conventions.AttributeCloudAccountID, "123")
					if testCase.wantResourceLogGroups != nil {
						assertStringArray(t, attrs, conventions.AttributeAWSLogGroupNames, testCase.wantResourceLogGroups[i])
					}
					if testCase.wantResourceLogStreams != nil {
						assertStringArray(t, attrs, conventions.AttributeAWSLogStreamNames, testCase.wantResourceLogStreams[i])
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
