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

func TestParseConsoleLineInvalidTrailingJSON(t *testing.T) {
	// Braces must be balanced for consoleLineRegex to even capture a trailing
	// JSON blob (it's matched by (\{.*\})?) - an unclosed brace like
	// `{"resource":` never reaches json.Unmarshal at all; it just gets
	// absorbed into the message text since the trailing group is optional.
	_, err := parseConsoleLine(`2026-07-06T22:56:21.989Z warn broken message {"resource": invalid}`)
	require.ErrorContains(t, err, "failed to parse trailing structured fields")
}

func TestProcess(t *testing.T) {
	jsonLine := `{"ts":"2026-07-06T22:56:21.989Z","level":"warn","msg":"Failed to scrape Prometheus endpoint",` +
		`"resource":{"k8s.pod.name":"otel-agent-qkvqj"},"otelcol.component.id":"receiver_creator"}`
	consoleLine := "2026-07-06T22:56:21.989Z\twarn\tinternal/transaction.go:127\tFailed to scrape Prometheus endpoint\t" +
		`{"resource":{"k8s.pod.name":"otel-agent-qkvqj"},"otelcol.component.id":"receiver_creator"}`
	consoleLineNoFields := "2026-07-06T22:56:21.989Z\tinfo\tCollector started"

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
					"caller":               "internal/transaction.go:127",
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
					"caller":               "internal/transaction.go:127",
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
