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
