// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestValidateDoesntChangeFields(t *testing.T) {
	traceID := entry.NewBodyField("traceID")
	spanID := entry.NewBodyField("spanID")
	traceFlags := entry.NewBodyField("traceFlags")
	parser := TraceParser{
		TraceID: configoptional.Some(TraceIDConfig{
			ParseFrom: &traceID,
		}),
		SpanID: configoptional.Some(SpanIDConfig{
			ParseFrom: &spanID,
		}),
		TraceFlags: configoptional.Some(TraceFlagsConfig{
			ParseFrom: &traceFlags,
		}),
	}
	err := parser.Validate()
	require.NoError(t, err)
	require.Equal(t, &traceID, parser.TraceID.Get().ParseFrom)
	require.Equal(t, &spanID, parser.SpanID.Get().ParseFrom)
	require.Equal(t, &traceFlags, parser.TraceFlags.Get().ParseFrom)
}

func TestValidateSetsDefaultFields(t *testing.T) {
	traceID := entry.NewBodyField("trace_id")
	spanID := entry.NewBodyField("span_id")
	traceFlags := entry.NewBodyField("trace_flags")
	parser := TraceParser{}
	err := parser.Validate()
	require.NoError(t, err)
	require.Equal(t, &traceID, parser.TraceID.Get().ParseFrom)
	require.Equal(t, &spanID, parser.SpanID.Get().ParseFrom)
	require.Equal(t, &traceFlags, parser.TraceFlags.Get().ParseFrom)
}
