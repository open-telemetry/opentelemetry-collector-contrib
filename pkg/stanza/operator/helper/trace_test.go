// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
