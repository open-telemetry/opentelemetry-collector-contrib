// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafkaexporter

import (
	"testing"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func ptr(i int) *int {
	return &i
}

func Test_RawMarshaler(t *testing.T) {
	tests := []struct {
		name          string
		countExpected *int
		logRecord     func() plog.LogRecord
		marshaled     sarama.ByteEncoder
		errorExpected bool
	}{
		{
			name: "string",
			logRecord: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Body().SetStringVal("foo")
				return lr
			},
			errorExpected: false,
			marshaled:     []byte("\"foo\""),
		},
		{
			name: "[]byte",
			logRecord: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Body().SetBytesVal(pcommon.NewImmutableByteSlice([]byte("foo")))
				return lr
			},
			errorExpected: false,
			marshaled:     []byte("foo"),
		},
		{
			name: "double",
			logRecord: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Body().SetDoubleVal(float64(1.64))
				return lr
			},
			errorExpected: false,
			marshaled:     []byte("1.64"),
		},
		{
			name: "int",
			logRecord: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Body().SetIntVal(int64(456))
				return lr
			},
			errorExpected: false,
			marshaled:     []byte("456"),
		},
		{
			name: "empty",
			logRecord: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				return lr
			},
			countExpected: ptr(0),
			errorExpected: false,
			marshaled:     []byte{},
		},
		{
			name: "bool",
			logRecord: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				lr.Body().SetBoolVal(false)
				return lr
			},
			errorExpected: false,
			marshaled:     []byte("false"),
		},
		{
			name: "slice",
			logRecord: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				slice := lr.Body().SetEmptySliceVal()
				slice.AppendEmpty().SetStringVal("foo")
				slice.AppendEmpty().SetStringVal("bar")
				slice.AppendEmpty().SetBoolVal(false)
				return lr
			},
			errorExpected: false,
			marshaled:     []byte(`["foo","bar",false]`),
		},
		{
			name: "map",
			logRecord: func() plog.LogRecord {
				lr := plog.NewLogRecord()
				m := lr.Body().SetEmptyMapVal()
				m.PutString("foo", "foo")
				m.PutString("bar", "bar")
				m.PutBool("foobar", false)
				return lr
			},
			errorExpected: false,
			marshaled:     []byte(`{"bar":"bar","foo":"foo","foobar":false}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := newRawMarshaler()
			logs := plog.NewLogs()
			lr := test.logRecord()
			lr.MoveTo(logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty())
			messages, err := r.Marshal(logs, "foo")
			if test.errorExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			countExpected := 1
			if test.countExpected != nil {
				countExpected = *test.countExpected
			}
			assert.Len(t, messages, countExpected)
			if countExpected > 0 {
				bytes, ok := messages[0].Value.(sarama.ByteEncoder)
				require.True(t, ok)
				assert.Equal(t, test.marshaled, bytes)
			}
		})
	}
}
