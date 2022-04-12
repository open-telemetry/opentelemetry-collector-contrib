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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestValidateDoesntChangeFields(t *testing.T) {
	traceID := entry.NewBodyField("traceID")
	spanID := entry.NewBodyField("spanID")
	traceFlags := entry.NewBodyField("traceFlags")
	parser := TraceParser{
		TraceID: &TraceIDConfig{
			ParseFrom: &traceID,
		},
		SpanID: &SpanIDConfig{
			ParseFrom: &spanID,
		},
		TraceFlags: &TraceFlagsConfig{
			ParseFrom: &traceFlags,
		},
	}
	err := parser.Validate()
	require.NoError(t, err)
	require.Equal(t, &traceID, parser.TraceID.ParseFrom)
	require.Equal(t, &spanID, parser.SpanID.ParseFrom)
	require.Equal(t, &traceFlags, parser.TraceFlags.ParseFrom)
}

func TestValidateSetsDefaultFields(t *testing.T) {
	traceID := entry.NewBodyField("trace_id")
	spanID := entry.NewBodyField("span_id")
	traceFlags := entry.NewBodyField("trace_flags")
	parser := TraceParser{}
	err := parser.Validate()
	require.NoError(t, err)
	require.Equal(t, &traceID, parser.TraceID.ParseFrom)
	require.Equal(t, &spanID, parser.SpanID.ParseFrom)
	require.Equal(t, &traceFlags, parser.TraceFlags.ParseFrom)
}

func TestPreserveFields(t *testing.T) {
	traceID := entry.NewBodyField("traceID")
	spanID := entry.NewBodyField("spanID")
	traceFlags := entry.NewBodyField("traceFlags")
	parser := TraceParser{
		TraceID: &TraceIDConfig{
			PreserveTo: &traceID,
		},
		SpanID: &SpanIDConfig{
			PreserveTo: &spanID,
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
		"traceID":    "480140f3d770a5ae32f0a22b6a812cff",
		"spanID":     "92c3792d54ba94f3",
		"traceFlags": "01",
	}, entry.Body)

	value, _ := hex.DecodeString("480140f3d770a5ae32f0a22b6a812cff")
	require.Equal(t, value, entry.TraceID)
	value, _ = hex.DecodeString("92c3792d54ba94f3")
	require.Equal(t, value, entry.SpanID)
	value, _ = hex.DecodeString("01")
	require.Equal(t, value, entry.TraceFlags)
}
