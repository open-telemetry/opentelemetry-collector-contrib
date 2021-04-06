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

package trace

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/testutil"
)

func TestDefaultParser(t *testing.T) {
	traceParserConfig := NewTraceParserConfig("")
	_, err := traceParserConfig.Build(testutil.NewBuildContext(t))
	require.NoError(t, err)
}

func TestTraceParserParse(t *testing.T) {
	cases := []struct {
		name           string
		inputRecord    map[string]interface{}
		expectedRecord map[string]interface{}
		expectErr      bool
		traceId        string
		spanId         string
		traceFlags     string
	}{
		{
			"AllFields",
			map[string]interface{}{
				"trace_id":    "480140f3d770a5ae32f0a22b6a812cff",
				"span_id":     "92c3792d54ba94f3",
				"trace_flags": "01",
			},
			map[string]interface{}{},
			false,
			"480140f3d770a5ae32f0a22b6a812cff",
			"92c3792d54ba94f3",
			"01",
		},
		{
			"WrongFields",
			map[string]interface{}{
				"traceId":    "480140f3d770a5ae32f0a22b6a812cff",
				"traceFlags": "01",
				"spanId":     "92c3792d54ba94f3",
			},
			map[string]interface{}{
				"traceId":    "480140f3d770a5ae32f0a22b6a812cff",
				"spanId":     "92c3792d54ba94f3",
				"traceFlags": "01",
			},
			false,
			"",
			"",
			"",
		},
		{
			"OnlyTraceId",
			map[string]interface{}{
				"trace_id": "480140f3d770a5ae32f0a22b6a812cff",
			},
			map[string]interface{}{},
			false,
			"480140f3d770a5ae32f0a22b6a812cff",
			"",
			"",
		},
		{
			"WrongTraceIdFormat",
			map[string]interface{}{
				"trace_id":    "foo_bar",
				"span_id":     "92c3792d54ba94f3",
				"trace_flags": "01",
			},
			map[string]interface{}{},
			true,
			"",
			"92c3792d54ba94f3",
			"01",
		},
		{
			"WrongTraceFlagFormat",
			map[string]interface{}{
				"trace_id":    "480140f3d770a5ae32f0a22b6a812cff",
				"span_id":     "92c3792d54ba94f3",
				"trace_flags": "foo_bar",
			},
			map[string]interface{}{},
			true,
			"480140f3d770a5ae32f0a22b6a812cff",
			"92c3792d54ba94f3",
			"",
		},
		{
			"AllFields",
			map[string]interface{}{
				"trace_id":    "480140f3d770a5ae32f0a22b6a812cff",
				"span_id":     "92c3792d54ba94f3",
				"trace_flags": "01",
			},
			map[string]interface{}{},
			false,
			"480140f3d770a5ae32f0a22b6a812cff",
			"92c3792d54ba94f3",
			"01",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			traceParserConfig := NewTraceParserConfig("")
			_, _ = traceParserConfig.Build(testutil.NewBuildContext(t))
			e := entry.New()
			e.Body = tc.inputRecord
			err := traceParserConfig.Parse(e)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expectedRecord, e.Body)
			traceId, _ := hex.DecodeString(tc.traceId)
			if len(tc.traceId) == 0 {
				require.Nil(t, e.TraceId)
			} else {
				require.Equal(t, traceId, e.TraceId)
			}
			spanId, _ := hex.DecodeString(tc.spanId)
			if len(tc.spanId) == 0 {
				require.Nil(t, e.SpanId)
			} else {
				require.Equal(t, spanId, e.SpanId)
			}
			traceFlags, _ := hex.DecodeString(tc.traceFlags)
			if len(tc.traceFlags) == 0 {
				require.Nil(t, e.TraceFlags)
			} else {
				require.Equal(t, traceFlags, e.TraceFlags)
			}
		})
	}
}
