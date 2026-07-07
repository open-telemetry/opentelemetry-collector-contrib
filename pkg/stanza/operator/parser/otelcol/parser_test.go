// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcol

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

func newTestParser(t *testing.T) *Parser {
	config := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := config.Build(set)
	require.NoError(t, err)
	return op.(*Parser)
}

func TestConfigBuild(t *testing.T) {
	config := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := config.Build(set)
	require.NoError(t, err)
	require.IsType(t, &Parser{}, op)
}

func TestJSONImplementations(t *testing.T) {
	require.Implements(t, (*operator.Operator)(nil), new(Parser))
}

func TestParser(t *testing.T) {
	cases := []struct {
		name   string
		input  *entry.Entry
		expect func(t *testing.T, actual *entry.Entry)
	}{
		{
			"simple_string_ts_and_warn",
			&entry.Entry{
				Body: `{"ts":"2026-07-06T22:56:21.989Z","level":"warn","msg":"Failed to scrape","resource":{"service.name":"otel-agent"},"caller":"main.go:12","otelcol.component.id":"receiver"}`,
			},
			func(t *testing.T, actual *entry.Entry) {
				expectedTime, err := time.Parse(time.RFC3339Nano, "2026-07-06T22:56:21.989Z")
				require.NoError(t, err)
				require.Equal(t, expectedTime, actual.Timestamp)
				require.Equal(t, entry.Warn, actual.Severity)
				require.Equal(t, "warn", actual.SeverityText)
				require.Equal(t, "Failed to scrape", actual.Body)
				require.Equal(t, map[string]any{"service.name": "otel-agent"}, actual.Resource)
				require.Equal(t, map[string]any{
					"caller":               "main.go:12",
					"otelcol.component.id": "receiver",
				}, actual.Attributes)
			},
		},
		{
			"float_ts_and_info",
			&entry.Entry{
				Body: `{"ts":1690000000.123,"level":"info","msg":"Success","resource":{"service.name":"otel-collector"}}`,
			},
			func(t *testing.T, actual *entry.Entry) {
				expectedTime := time.Unix(1690000000, 123000000)
				require.Equal(t, expectedTime, actual.Timestamp)
				require.Equal(t, entry.Info, actual.Severity)
				require.Equal(t, "info", actual.SeverityText)
				require.Equal(t, "Success", actual.Body)
				require.Equal(t, map[string]any{"service.name": "otel-collector"}, actual.Resource)
				require.Empty(t, actual.Attributes)
			},
		},
		{
			"missing_fields",
			&entry.Entry{
				Body: `{"caller":"main.go:12"}`,
			},
			func(t *testing.T, actual *entry.Entry) {
				require.True(t, actual.Timestamp.IsZero())
				require.Equal(t, entry.Default, actual.Severity)
				require.Empty(t, actual.SeverityText)
				require.Equal(t, `{"caller":"main.go:12"}`, actual.Body) // ParseWith does not delete msg/ts since they aren't there, body is not changed if msg not found
				require.Nil(t, actual.Resource)
				require.Equal(t, map[string]any{
					"caller": "main.go:12",
				}, actual.Attributes)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestParser(t)
			err := p.Process(t.Context(), tc.input)
			require.NoError(t, err)
			tc.expect(t, tc.input)
		})
	}
}
