// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloudexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	"google.golang.org/genproto/googleapis/logging/v2"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter/testhelpers"
)

func TestBuildEntry(t *testing.T) {
	testCases := []struct {
		name          string
		otelRecord    *pdata.LogRecord
		builder       GoogleEntryBuilder
		expectedEntry *logging.LogEntry
		expectedErr   string
	}{
		{
			name:       "log record body empty",
			otelRecord: testhelpers.NewLogRecordBuilder().Build(),
			builder: GoogleEntryBuilder{
				MaxEntrySize: 1000,
			},
			expectedErr: "failed to set payload: cannot convert record of type EMPTY",
		},
		{
			name:       "nameFields incorrect final field format",
			otelRecord: testhelpers.NewLogRecordBuilder().WithBodyString("body").Build(),
			builder: GoogleEntryBuilder{
				MaxEntrySize: 1000,
				NameFields:   []string{"key1-3", "key1-2.key2-2"},
			},
			expectedErr: "failed to set log name: failed to read log name field: name value can't be of type: MAP",
		},
		{
			name:       "nameFields incorrect nesting format",
			otelRecord: testhelpers.NewLogRecordBuilder().WithBodyString("body").Build(),
			builder: GoogleEntryBuilder{
				MaxEntrySize: 1000,
				NameFields:   []string{"key1-1.key2-1"},
			},
			expectedErr: "failed to set log name: failed to read log name field: key: key1-1 value must be of type: MAP but instead is: STRING",
		},
		{
			name:       "log entry size too large",
			otelRecord: testhelpers.NewLogRecordBuilder().WithBodyString("body").Build(),
			builder: GoogleEntryBuilder{
				MaxEntrySize: 10,
				NameFields:   []string{"key1-1"},
			},
			expectedErr: "exceeds max entry size: ",
		},
		{
			name: "valid string entry",
			otelRecord: testhelpers.NewLogRecordBuilder().
				WithBodyString("test").
				WithTimeStamp().
				WithSpanID([...]byte{0, 1, 0, 0, 0, 0, 0, 0}).
				WithTraceID([...]byte{0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}).
				Build(),
			builder: GoogleEntryBuilder{
				ProjectID:    "test_project",
				MaxEntrySize: 10000,
				NameFields:   []string{"key1-1"},
			},
			expectedEntry: &logging.LogEntry{
				LogName: "projects/test_project/logs/val1",
				Payload: &logging.LogEntry_TextPayload{
					TextPayload: "test",
				},
				Labels: map[string]string{
					"key1-2.key2-1":        "val2",
					"key1-2.key2-2.key3-1": "val3",
				},
				Trace:     "00000100000000000000000000000000",
				SpanId:    "0001000000000000",
				Timestamp: timestamppb.New(time.UnixMilli(0)),
			},
		},
		{
			name: "valid bytes entry with no attributes",
			otelRecord: testhelpers.NewLogRecordBuilder().
				WithBodyBytes("test").
				WithTimeStamp().
				WithEmptyAttributes().
				WithSpanID([...]byte{0, 1, 0, 0, 0, 0, 0, 0}).
				WithTraceID([...]byte{0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}).
				Build(),
			builder: GoogleEntryBuilder{
				ProjectID:    "test_project",
				MaxEntrySize: 10000,
				NameFields:   []string{"key1-1"},
			},
			expectedEntry: &logging.LogEntry{
				LogName: "projects/test_project/logs/",
				Payload: &logging.LogEntry_TextPayload{
					TextPayload: "test",
				},
				Trace:     "00000100000000000000000000000000",
				SpanId:    "0001000000000000",
				Timestamp: timestamppb.New(time.UnixMilli(0)),
			},
		},
		{
			name: "valid map entry",
			otelRecord: testhelpers.NewLogRecordBuilder().
				WithBodyMap().
				WithTimeStamp().
				WithSpanID([...]byte{0, 1, 0, 0, 0, 0, 0, 0}).
				WithTraceID([...]byte{0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}).
				Build(),
			builder: GoogleEntryBuilder{
				ProjectID:    "test_project",
				MaxEntrySize: 10000,
				NameFields:   []string{"key1-1"},
			},
			expectedEntry: &logging.LogEntry{
				LogName: "projects/test_project/logs/val1",
				Payload: &logging.LogEntry_JsonPayload{
					JsonPayload: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"key1": structpb.NewStringValue("body1"),
							"key2": structpb.NewNumberValue(10),
							"key3": structpb.NewBoolValue(true),
						},
					},
				},
				Labels: map[string]string{
					"key1-2.key2-1":        "val2",
					"key1-2.key2-2.key3-1": "val3",
				},
				Trace:     "00000100000000000000000000000000",
				SpanId:    "0001000000000000",
				Timestamp: timestamppb.New(time.UnixMilli(0)),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entry, err := tc.builder.Build(tc.otelRecord)
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedEntry.String(), entry.String())
		})
	}
}
