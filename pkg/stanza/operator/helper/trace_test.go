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
