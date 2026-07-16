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

func newTestParserWithFormat(t *testing.T, format string) *Parser {
	config := NewConfigWithID("test")
	config.Format = format
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

func TestConfigBuildDefaultFormat(t *testing.T) {
	parser := newTestParser(t)
	require.Equal(t, formatAuto, parser.format)
}

func TestConfigBuildInvalidFormat(t *testing.T) {
	config := NewConfigWithID("test")
	config.Format = "yaml"
	set := componenttest.NewNopTelemetrySettings()
	_, err := config.Build(set)
	require.ErrorContains(t, err, "invalid `format` field")
}

func TestParseInvalidType(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse(12345)
	require.ErrorContains(t, err, "cannot be parsed as an otelcol self-log")
}

func TestParseEmptyLine(t *testing.T) {
	parser := newTestParser(t)
	_, err := parser.parse("   ")
	require.ErrorContains(t, err, "empty line")
}

func TestParseUnknownFormat(t *testing.T) {
	parser := newTestParser(t)
	parser.format = "bogus"
	_, err := parser.parse(`{"ts":"2026-01-01T00:00:00Z"}`)
	require.ErrorContains(t, err, "unknown otelcol self-log format")
}

func TestDetectFormat(t *testing.T) {
	require.Equal(t, formatJSON, detectFormat(`{"ts":"2026-01-01T00:00:00Z"}`))
	require.Equal(t, formatConsole, detectFormat("2026-01-01T00:00:00Z info started"))
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
		name           string
		rest           string
		wantMsg        string
		wantFields     map[string]any
		wantMalformed  bool
	}{
		{
			name:          "no_trailing_fields",
			rest:          "Collector started",
			wantMsg:       "Collector started",
			wantFields:    nil,
			wantMalformed: false,
		},
		{
			name:          "simple_trailing_fields",
			rest:          `Failed to scrape endpoint {"otelcol.component.id":"receiver_creator"}`,
			wantMsg:       "Failed to scrape endpoint",
			wantFields:    map[string]any{"otelcol.component.id": "receiver_creator"},
			wantMalformed: false,
		},
		{
			name:          "nested_trailing_fields",
			rest:          `Failed to scrape endpoint {"resource":{"k8s.pod.name":"otel-agent-qkvqj"}}`,
			wantMsg:       "Failed to scrape endpoint",
			wantFields:    map[string]any{"resource": map[string]any{"k8s.pod.name": "otel-agent-qkvqj"}},
			wantMalformed: false,
		},
		{
			name:          "brace_in_message_before_real_fields",
			rest:          `Config value for pool {default} is missing {"resource":{"k8s.pod.name":"otel-agent-qkvqj"}}`,
			wantMsg:       "Config value for pool {default} is missing",
			wantFields:    map[string]any{"resource": map[string]any{"k8s.pod.name": "otel-agent-qkvqj"}},
			wantMalformed: false,
		},
		{
			name:          "empty_object_in_message_before_real_fields",
			rest:          `Cache miss for key {} retrying {"otelcol.signal":"logs"}`,
			wantMsg:       "Cache miss for key {} retrying",
			wantFields:    map[string]any{"otelcol.signal": "logs"},
			wantMalformed: false,
		},
		{
			name: "many_braces_before_real_fields",
			rest: `Pool stats: {a} {b} {c} {d} {e} retrying ` +
				`{"resource":{"k8s.pod.name":"otel-agent-qkvqj"},"otelcol.component.id":"receiver_creator"}`,
			wantMsg: "Pool stats: {a} {b} {c} {d} {e} retrying",
			wantFields: map[string]any{
				"resource":              map[string]any{"k8s.pod.name": "otel-agent-qkvqj"},
				"otelcol.component.id": "receiver_creator",
			},
			wantMalformed: false,
		},
		{
			// A brace in the message with no real trailing JSON at all -
			// degrades gracefully, and is correctly flagged as malformed
			// since a "{" was seen but never resolved.
			name:          "brace_in_message_no_real_fields",
			rest:          "Value set to {ok}",
			wantMsg:       "Value set to {ok}",
			wantFields:    nil,
			wantMalformed: true,
		},
		{
			// Malformed trailing JSON degrades gracefully rather than
			// erroring, but is flagged as malformed so the loss is visible.
			name:          "malformed_trailing_fields",
			rest:          `broken message {"resource": invalid}`,
			wantMsg:       `broken message {"resource": invalid}`,
			wantFields:    nil,
			wantMalformed: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, fields, malformed := splitConsoleMessageAndFields(tc.rest)
			require.Equal(t, tc.wantMsg, msg)
			require.Equal(t, tc.wantFields, fields)
			require.Equal(t, tc.wantMalformed, malformed)
		})
	}
}

func TestProcess(t *testing.T) {
	jsonLine := `{"ts":"2026-07-06T22:56:21.989Z","level":"warn","msg":"Failed to scrape Prometheus endpoint",` +
		`"resource":{"k8s.pod.name":"otel-agent-qkvqj"},"otelcol.component.id":"receiver_creator"}`
	consoleLine := "2026-07-06T22:56:21.989Z\twarn\tinternal/transaction.go:127\tFailed to scrape Prometheus endpoint\t" +
		`{"resource":{"k8s.pod.name":"otel-agent-qkvqj"},"otelcol.component.id":"receiver_creator"}`
	consoleLineNoFields := "2026-07-06T22:56:21.989Z\tinfo\tCollector started"
	consoleLineBraceInMessage := "2026-07-06T22:56:21.989Z\twarn\tConfig value for pool {default} is missing\t" +
		`{"resource":{"k8s.pod.name":"otel-agent-qkvqj"}}`
	consoleLineMalformedFields := "2026-07-06T22:56:21.989Z\twarn\tbroken message\t{\"resource\": invalid}"

	wantTimestamp := time.Date(2026, time.July, 6, 22, 56, 21, 989000000, time.UTC)

	cases := []struct {
		name   string
		format string
		input  *entry.Entry
		expect *entry.Entry
	}{
		{
			name:   "json_forced",
			format: formatJSON,
			input:  &entry.Entry{Body: jsonLine},
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
			name:   "json_auto_detected",
			format: formatAuto,
			input:  &entry.Entry{Body: jsonLine},
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
			name:   "console_forced",
			format: formatConsole,
			input:  &entry.Entry{Body: consoleLine},
			expect: &entry.Entry{
				Timestamp:    wantTimestamp,
				Severity:     entry.Warn,
				SeverityText: "warn",
				Body:         "Failed to scrape Prometheus endpoint",
				Resource:     map[string]any{"k8s.pod.name": "otel-agent-qkvqj"},
				Attributes: map[string]any{
					"caller":                "internal/transaction.go:127",
					"otelcol.component.id": "receiver_creator",
				},
			},
		},
		{
			name:   "console_auto_detected",
			format: formatAuto,
			input:  &entry.Entry{Body: consoleLine},
			expect: &entry.Entry{
				Timestamp:    wantTimestamp,
				Severity:     entry.Warn,
				SeverityText: "warn",
				Body:         "Failed to scrape Prometheus endpoint",
				Resource:     map[string]any{"k8s.pod.name": "otel-agent-qkvqj"},
				Attributes: map[string]any{
					"caller":                "internal/transaction.go:127",
					"otelcol.component.id": "receiver_creator",
				},
			},
		},
		{
			name:   "console_no_structured_fields",
			format: formatConsole,
			input:  &entry.Entry{Body: consoleLineNoFields},
			expect: &entry.Entry{
				Timestamp:    wantTimestamp,
				Severity:     entry.Info,
				SeverityText: "info",
				Body:         "Collector started",
				Attributes:   map[string]any{},
			},
		},
		{
			name:   "console_brace_in_message",
			format: formatConsole,
			input:  &entry.Entry{Body: consoleLineBraceInMessage},
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
			// Checkbox: malformed trailing JSON surfaces as an attribute on
			// the final entry, not just internally in the split function.
			name:   "console_malformed_fields_attribute",
			format: formatConsole,
			input:  &entry.Entry{Body: consoleLineMalformedFields},
			expect: &entry.Entry{
				Timestamp:    wantTimestamp,
				Severity:     entry.Warn,
				SeverityText: "warn",
				Body:         "broken message\t{\"resource\": invalid}",
				Attributes:   map[string]any{malformedTrailingFieldsAttr: true},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := newTestParserWithFormat(t, tc.format)
			err := parser.Process(t.Context(), tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expect, tc.input)
		})
	}
}

func TestProcessFailure(t *testing.T) {
	cases := []struct {
		name           string
		format         string
		input          *entry.Entry
		expectedErrMsg string
	}{
		{
			name:           "malformed_json",
			format:         formatJSON,
			input:          &entry.Entry{Body: `{"ts":`},
			expectedErrMsg: "cannot be parsed as a json-encoded otelcol self-log",
		},
		{
			name:           "malformed_console",
			format:         formatConsole,
			input:          &entry.Entry{Body: "singleword"},
			expectedErrMsg: "cannot be parsed as a console-encoded otelcol self-log",
		},
		{
			name:           "empty_body",
			format:         formatAuto,
			input:          &entry.Entry{Body: ""},
			expectedErrMsg: "empty line",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := newTestParserWithFormat(t, tc.format)
			err := parser.Process(t.Context(), tc.input)
			require.ErrorContains(t, err, tc.expectedErrMsg)
		})
	}
}

func TestProcessBatch(t *testing.T) {
	ctx := t.Context()
	parser := newTestParserWithFormat(t, formatJSON)
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