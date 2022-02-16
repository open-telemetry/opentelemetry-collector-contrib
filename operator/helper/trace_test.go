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

package helper

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
)

func TestValidateDoesntChangeFields(t *testing.T) {
	traceId := entry.NewBodyField("traceId")
	spanId := entry.NewBodyField("spanId")
	traceFlags := entry.NewBodyField("traceFlags")
	parser := TraceParser{
		TraceId: &TraceIdConfig{
			ParseFrom: &traceId,
		},
		SpanId: &SpanIdConfig{
			ParseFrom: &spanId,
		},
		TraceFlags: &TraceFlagsConfig{
			ParseFrom: &traceFlags,
		},
	}
	err := parser.Validate()
	require.NoError(t, err)
	require.Equal(t, &traceId, parser.TraceId.ParseFrom)
	require.Equal(t, &spanId, parser.SpanId.ParseFrom)
	require.Equal(t, &traceFlags, parser.TraceFlags.ParseFrom)
}

func TestValidateSetsDefaultFields(t *testing.T) {
	traceId := entry.NewBodyField("trace_id")
	spanId := entry.NewBodyField("span_id")
	traceFlags := entry.NewBodyField("trace_flags")
	parser := TraceParser{}
	err := parser.Validate()
	require.NoError(t, err)
	require.Equal(t, &traceId, parser.TraceId.ParseFrom)
	require.Equal(t, &spanId, parser.SpanId.ParseFrom)
	require.Equal(t, &traceFlags, parser.TraceFlags.ParseFrom)
}

func TestPreserveFields(t *testing.T) {
	traceId := entry.NewBodyField("traceId")
	spanId := entry.NewBodyField("spanId")
	traceFlags := entry.NewBodyField("traceFlags")
	parser := TraceParser{
		TraceId: &TraceIdConfig{
			PreserveTo: &traceId,
		},
		SpanId: &SpanIdConfig{
			PreserveTo: &spanId,
		},
		TraceFlags: &TraceFlagsConfig{
			PreserveTo: &traceFlags,
		},
	}
	err := parser.Validate()
	require.NoError(t, err)

	entry := entry.New()
	entry.Body = map[string]interface{}{
		"trace_id":    "480140f3d770a5ae32f0a22b6a812cff",
		"span_id":     "92c3792d54ba94f3",
		"trace_flags": "01",
	}
	err = parser.Parse(entry)
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{
		"traceId":    "480140f3d770a5ae32f0a22b6a812cff",
		"spanId":     "92c3792d54ba94f3",
		"traceFlags": "01",
	}, entry.Body)

	value, _ := hex.DecodeString("480140f3d770a5ae32f0a22b6a812cff")
	require.Equal(t, value, entry.TraceId)
	value, _ = hex.DecodeString("92c3792d54ba94f3")
	require.Equal(t, value, entry.SpanId)
	value, _ = hex.DecodeString("01")
	require.Equal(t, value, entry.TraceFlags)
}
