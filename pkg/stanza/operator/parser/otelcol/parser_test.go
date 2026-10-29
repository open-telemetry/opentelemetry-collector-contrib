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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
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

// TestConfigBuildUsesFixedSchema asserts the operator always reads the body and
// writes to the attributes root, and that none of the parser-helper behaviors a
// user could otherwise configure are active.
func TestConfigBuildUsesFixedSchema(t *testing.T) {
	parser := newTestParser(t)

	require.Equal(t, entry.NewBodyField(), parser.ParseFrom)
	require.Equal(t, entry.NewAttributeField(), parser.ParseTo)
	require.Nil(t, parser.BodyField)
	require.Nil(t, parser.TimeParser)
	require.Nil(t, parser.SeverityParser)
	require.Nil(t, parser.TraceParser)
	require.Nil(t, parser.ScopeNameParser)

	e := entry.New()
	e.Body = `{"ts":"2026-07-06T22:56:21.989Z","level":"info","msg":"hello","extra":"value"}`

	require.NoError(t, parser.Process(t.Context(), e))
	require.Equal(t, "hello", e.Body)
	require.Equal(t, "value", e.Attributes["extra"])
}

func TestParseInvalidType(t *testing.T) {
	_, err := parseLine(12345)
	require.ErrorContains(t, err, "cannot be parsed as an otelcol self-log")
}

func TestParseEmptyLine(t *testing.T) {
	_, err := parseLine("   ")
	require.ErrorContains(t, err, "empty line")
}

func TestIsJSONEncoded(t *testing.T) {
	require.True(t, isJSONEncoded(`{"ts":"2026-01-01T00:00:00Z"}`))
	require.False(t, isJSONEncoded("2026-01-01T00:00:00Z info started"))
}

func TestParseJSONLineInvalid(t *testing.T) {
	_, err := parseJSONLine(`{"ts":`)
	require.ErrorContains(t, err, "cannot be parsed as a json-encoded otelcol self-log")
}

func TestParseConsoleLineInvalid(t *testing.T) {
	_, err := parseConsoleLine("singleword")
	require.ErrorContains(t, err, "cannot be parsed as a console-encoded otelcol self-log")
}

// --- splitConsoleMessageAndFields: direct unit tests ---

func TestSplitConsoleMessageAndFields(t *testing.T) {
	cases := []struct {
		name       string
		rest       string
		wantMsg    string
		wantFields map[string]any
	}{
		{
			name:       "no_trailing_fields",
			rest:       "Collector started",
			wantMsg:    "Collector started",
			wantFields: nil,
		},
		{
			name:       "simple_trailing_fields",
			rest:       `Failed to scrape endpoint {"otelcol.component.id":"receiver_creator"}`,
			wantMsg:    "Failed to scrape endpoint",
			wantFields: map[string]any{"otelcol.component.id": "receiver_creator"},
		},
		{
			name:       "nested_trailing_fields",
			rest:       `Failed to scrape endpoint {"resource":{"k8s.pod.name":"otel-agent-qkvqj"}}`,
			wantMsg:    "Failed to scrape endpoint",
			wantFields: map[string]any{"resource": map[string]any{"k8s.pod.name": "otel-agent-qkvqj"}},
		},
		{
			name:       "brace_in_message_before_real_fields",
			rest:       `Config value for pool {default} is missing {"resource":{"k8s.pod.name":"otel-agent-qkvqj"}}`,
			wantMsg:    "Config value for pool {default} is missing",
			wantFields: map[string]any{"resource": map[string]any{"k8s.pod.name": "otel-agent-qkvqj"}},
		},
		{
			// "{}" alone is valid JSON, so this checks the scan doesn't stop early on it.
			name:       "empty_object_in_message_before_real_fields",
			rest:       `Cache miss for key {} retrying {"otelcol.signal":"logs"}`,
			wantMsg:    "Cache miss for key {} retrying",
			wantFields: map[string]any{"otelcol.signal": "logs"},
		},
		{
			name: "many_braces_before_real_fields",
			rest: `Pool stats: {a} {b} {c} {d} {e} retrying ` +
				`{"resource":{"k8s.pod.name":"otel-agent-qkvqj"},"otelcol.component.id":"receiver_creator"}`,
			wantMsg: "Pool stats: {a} {b} {c} {d} {e} retrying",
			wantFields: map[string]any{
				"resource":             map[string]any{"k8s.pod.name": "otel-agent-qkvqj"},
				"otelcol.component.id": "receiver_creator",
			},
		},
		{
			name:       "brace_in_message_no_real_fields",
			rest:       "Value set to {ok}",
			wantMsg:    "Value set to {ok}",
			wantFields: nil,
		},
		{
			// Does not end in "}", so this exercises the early-exit path.
			name:       "brace_in_message_not_at_end",
			rest:       "Config value for pool {default} is missing",
			wantMsg:    "Config value for pool {default} is missing",
			wantFields: nil,
		},
		{
			name:       "malformed_trailing_fields",
			rest:       `broken message {"resource": invalid}`,
			wantMsg:    `broken message {"resource": invalid}`,
			wantFields: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, fields := splitConsoleMessageAndFields(tc.rest)
			require.Equal(t, tc.wantMsg, msg)
			require.Equal(t, tc.wantFields, fields)
		})
	}
}

func TestProcess(t *testing.T) {
	jsonLine := `{"ts":"2026-07-06T22:56:21.989Z","level":"warn","msg":"Failed to scrape Prometheus endpoint",` +
		`"resource":{"k8s.pod.name":"otel-agent-qkvqj"},"otelcol.component.id":"receiver_creator"}`
	jsonLineWithCaller := `{"ts":"2026-07-06T22:56:21.989Z","level":"warn","caller":"internal/transaction.go:127",` +
		`"msg":"Failed to scrape Prometheus endpoint"}`
	consoleLine := "2026-07-06T22:56:21.989Z\twarn\tinternal/transaction.go:127\tFailed to scrape Prometheus endpoint\t" +
		`{"resource":{"k8s.pod.name":"otel-agent-qkvqj"},"otelcol.component.id":"receiver_creator"}`
	consoleLineNoFields := "2026-07-06T22:56:21.989Z\tinfo\tCollector started"
	consoleLineBraceInMessage := "2026-07-06T22:56:21.989Z\twarn\tConfig value for pool {default} is missing\t" +
		`{"resource":{"k8s.pod.name":"otel-agent-qkvqj"}}`
	consoleLineMalformedFields := "2026-07-06T22:56:21.989Z\twarn\tbroken message\t{\"resource\": invalid}"

	wantTimestamp := time.Date(2026, time.July, 6, 22, 56, 21, 989000000, time.UTC)

	cases := []struct {
		name   string
		input  *entry.Entry
		expect *entry.Entry
	}{
		{
			name:  "json",
			input: &entry.Entry{Body: jsonLine},
			expect: &entry.Entry{
				Timestamp:    wantTimestamp,
				Severity:     entry.Warn,
				SeverityText: "warn",
				Body:         "Failed to scrape Prometheus endpoint",
				Resource:     map[string]any{"k8s.pod.name": "otel-agent-qkvqj"},
				Attributes:   map[string]any{"otelcol.component.id": "receiver_creator"},
			},
		},
		{
			// zap puts the caller in the line prefix in console encoding and in a
			// "caller" key in json encoding; either way it stays an attribute.
			name:  "json_with_caller",
			input: &entry.Entry{Body: jsonLineWithCaller},
			expect: &entry.Entry{
				Timestamp:    wantTimestamp,
				Severity:     entry.Warn,
				SeverityText: "warn",
				Body:         "Failed to scrape Prometheus endpoint",
				Attributes:   map[string]any{"caller": "internal/transaction.go:127"},
			},
		},
		{
			name:  "console",
			input: &entry.Entry{Body: consoleLine},
			expect: &entry.Entry{
				Timestamp:    wantTimestamp,
				Severity:     entry.Warn,
				SeverityText: "warn",
				Body:         "Failed to scrape Prometheus endpoint",
				Resource:     map[string]any{"k8s.pod.name": "otel-agent-qkvqj"},
				Attributes: map[string]any{
					"caller":               "internal/transaction.go:127",
					"otelcol.component.id": "receiver_creator",
				},
			},
		},
		{
			name:  "console_no_structured_fields",
			input: &entry.Entry{Body: consoleLineNoFields},
			expect: &entry.Entry{
				Timestamp:    wantTimestamp,
				Severity:     entry.Info,
				SeverityText: "info",
				Body:         "Collector started",
				Attributes:   map[string]any{},
			},
		},
		{
			name:  "console_brace_in_message",
			input: &entry.Entry{Body: consoleLineBraceInMessage},
			expect: &entry.Entry{
				Timestamp:    wantTimestamp,
				Severity:     entry.Warn,
				SeverityText: "warn",
				Body:         "Config value for pool {default} is missing",
				Resource:     map[string]any{"k8s.pod.name": "otel-agent-qkvqj"},
				Attributes:   map[string]any{},
			},
		},
		{
			name:  "console_malformed_fields_degrades_gracefully",
			input: &entry.Entry{Body: consoleLineMalformedFields},
			expect: &entry.Entry{
				Timestamp:    wantTimestamp,
				Severity:     entry.Warn,
				SeverityText: "warn",
				Body:         "broken message\t{\"resource\": invalid}",
				Attributes:   map[string]any{},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := newTestParser(t)
			err := parser.Process(t.Context(), tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expect, tc.input)
		})
	}
}

func TestProcessFailure(t *testing.T) {
	cases := []struct {
		name           string
		input          *entry.Entry
		expectedErrMsg string
	}{
		{
			name:           "malformed_json",
			input:          &entry.Entry{Body: `{"ts":`},
			expectedErrMsg: "cannot be parsed as a json-encoded otelcol self-log",
		},
		{
			name:           "malformed_console",
			input:          &entry.Entry{Body: "singleword"},
			expectedErrMsg: "cannot be parsed as a console-encoded otelcol self-log",
		},
		{
			name:           "empty_body",
			input:          &entry.Entry{Body: ""},
			expectedErrMsg: "empty line",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := newTestParser(t)
			err := parser.Process(t.Context(), tc.input)
			require.ErrorContains(t, err, tc.expectedErrMsg)
		})
	}
}

// A "ts" the parser can't interpret must not be silently dropped: it stays in
// the attributes so the raw value is still shipped, and the error is surfaced
// through on_error so the misconfiguration is visible.
func TestProcessUninterpretableTimestampIsPreserved(t *testing.T) {
	cases := []struct {
		name           string
		body           string
		expectedErrMsg string
		expectedTS     any
	}{
		{
			name:           "unknown_layout",
			body:           `{"ts":"not-a-timestamp","level":"warn","msg":"Failed to scrape endpoint"}`,
			expectedErrMsg: `value "not-a-timestamp" did not match any known layout`,
			expectedTS:     "not-a-timestamp",
		},
		{
			name:           "unsupported_type",
			body:           `{"ts":true,"level":"warn","msg":"Failed to scrape endpoint"}`,
			expectedErrMsg: `unsupported type bool for "ts" field`,
			expectedTS:     true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := newTestParser(t)
			e := &entry.Entry{Body: tc.body}

			err := parser.Process(t.Context(), e)
			require.ErrorContains(t, err, tc.expectedErrMsg)

			require.True(t, e.Timestamp.IsZero())
			require.Equal(t, tc.expectedTS, e.Attributes["ts"])

			// Everything the parser could interpret is still promoted.
			require.Equal(t, entry.Warn, e.Severity)
			require.Equal(t, "warn", e.SeverityText)
			require.Equal(t, "Failed to scrape endpoint", e.Body)
		})
	}
}

func TestProcessBatch(t *testing.T) {
	ctx := t.Context()
	parser := newTestParser(t)
	fake := testutil.NewFakeOutput(t)
	parser.OutputOperators = []operator.Operator{fake}

	input := []*entry.Entry{
		{Body: `{"ts":"2026-07-06T22:56:21.989Z","level":"info","msg":"first"}`},
		{Body: `{"ts":"2026-07-06T22:56:22.989Z","level":"info","msg":"second"}`},
	}

	require.NoError(t, parser.ProcessBatch(ctx, input))

	fake.ExpectEntries(t, []*entry.Entry{
		{
			Timestamp:    time.Date(2026, time.July, 6, 22, 56, 21, 989000000, time.UTC),
			Severity:     entry.Info,
			SeverityText: "info",
			Body:         "first",
			Attributes:   map[string]any{},
		},
		{
			Timestamp:    time.Date(2026, time.July, 6, 22, 56, 22, 989000000, time.UTC),
			Severity:     entry.Info,
			SeverityText: "info",
			Body:         "second",
			Attributes:   map[string]any{},
		},
	})

	select {
	case e := <-fake.Received:
		require.FailNow(t, "Received unexpected entry: ", "%+v", e)
	default:
	}
}
