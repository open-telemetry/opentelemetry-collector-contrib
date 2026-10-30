// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcol

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

const loggerName = "prw.wal"

func newTestParser(t *testing.T) *Parser {
	config := NewConfigWithID("test")
	set := componenttest.NewNopTelemetrySettings()
	op, err := config.Build(set)
	require.NoError(t, err)
	return op.(*Parser)
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

// TestParseConsoleLine covers the console field layouts the parser has to tell
// apart, since the logger and caller fields are optional and sit directly in
// front of the message.
func TestParseConsoleLine(t *testing.T) {
	const ts = "2026-07-06T22:56:21.989Z"

	cases := []struct {
		name string
		line string
		want map[string]any
	}{
		{
			name: "caller_and_fields",
			line: ts + "\twarn\tinternal/transaction.go:127\tFailed to scrape endpoint\t" +
				`{"otelcol.component.id":"receiver_creator"}`,
			want: map[string]any{
				"ts": ts, "level": "warn", "caller": "internal/transaction.go:127",
				"msg": "Failed to scrape endpoint", "otelcol.component.id": "receiver_creator",
			},
		},
		{
			name: "logger_and_caller",
			line: ts + "\twarn\tprw.wal\tinternal/transaction.go:127\tFailed to scrape endpoint",
			want: map[string]any{
				"ts": ts, "level": "warn", "logger": "prw.wal",
				"caller": "internal/transaction.go:127", "msg": "Failed to scrape endpoint",
			},
		},
		{
			// disable_caller: the first word of the message must not be taken
			// for a caller just because it ends in ":<number>".
			name: "no_caller_message_looks_like_caller",
			line: ts + "\tinfo\tlocalhost:4317 refused the connection",
			want: map[string]any{"ts": ts, "level": "info", "msg": "localhost:4317 refused the connection"},
		},
		{
			name: "no_caller_with_logger",
			line: ts + "\tinfo\tprw.wal\tFailed to scrape endpoint",
			want: map[string]any{"ts": ts, "level": "info", "logger": "prw.wal", "msg": "Failed to scrape endpoint"},
		},
		{
			name: "no_caller_no_logger_no_fields",
			line: ts + "\tinfo\tCollector started",
			want: map[string]any{"ts": ts, "level": "info", "msg": "Collector started"},
		},
		{
			name: "empty_message",
			line: ts + "\tinfo\tinternal/transaction.go:127\t\t" + `{"otelcol.signal":"logs"}`,
			want: map[string]any{
				"ts": ts, "level": "info", "caller": "internal/transaction.go:127",
				"msg": "", "otelcol.signal": "logs",
			},
		},
		{
			name: "braces_in_message",
			line: ts + "\twarn\tConfig value for pool {default} is missing\t" +
				`{"resource":{"k8s.pod.name":"otel-agent-qkvqj"}}`,
			want: map[string]any{
				"ts": ts, "level": "warn", "msg": "Config value for pool {default} is missing",
				"resource": map[string]any{"k8s.pod.name": "otel-agent-qkvqj"},
			},
		},
		{
			// The message is only separated from the fields by the tab, so a
			// message that is itself a JSON object stays the message.
			name: "json_object_in_message",
			line: ts + "\tinfo\t" + `received {"a":1}` + "\t" + `{"otelcol.signal":"logs"}`,
			want: map[string]any{
				"ts": ts, "level": "info", "msg": `received {"a":1}`, "otelcol.signal": "logs",
			},
		},
		{
			name: "malformed_fields_stay_in_message",
			line: ts + "\twarn\tbroken message\t" + `{"resource": invalid}`,
			want: map[string]any{"ts": ts, "level": "warn", "msg": "broken message\t" + `{"resource": invalid}`},
		},
		{
			name: "tab_in_message",
			line: ts + "\tinfo\tinternal/transaction.go:127\tcolumns:\ta\tb\t" + `{"otelcol.signal":"logs"}`,
			want: map[string]any{
				"ts": ts, "level": "info", "caller": "internal/transaction.go:127",
				"msg": "columns:\ta\tb", "otelcol.signal": "logs",
			},
		},
		{
			name: "stacktrace",
			line: ts + "\terror\tinternal/transaction.go:127\tExporting failed\t" +
				`{"error":"connection refused"}` + "\nmain.main\n\t/src/main.go:61",
			want: map[string]any{
				"ts": ts, "level": "error", "caller": "internal/transaction.go:127",
				"msg": "Exporting failed", "error": "connection refused",
				"stacktrace": "main.main\n\t/src/main.go:61",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseConsoleLine(tc.line)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestParseZapOutput feeds the parser output from a real zap logger built the
// way the collector builds it, across every service::telemetry::logs setting
// that changes the encoded layout.
func TestParseZapOutput(t *testing.T) {
	for _, encoding := range []string{"console", "json"} {
		for _, disableCaller := range []bool{false, true} {
			for _, named := range []bool{false, true} {
				for _, stacktrace := range []bool{false, true} {
					name := fmt.Sprintf("%s/disable_caller=%v/named=%v/stacktrace=%v",
						encoding, disableCaller, named, stacktrace)
					t.Run(name, func(t *testing.T) {
						msg := "Failed to scrape endpoint"
						if !disableCaller {
							// Exercise the ambiguity the caller field creates.
							msg = "localhost:4317 refused the connection"
						}
						line := encodeWithZap(t, encoding, disableCaller, named, stacktrace, msg)

						parsed, err := parseLine(line)
						require.NoError(t, err)
						m, ok := parsed.(map[string]any)
						require.True(t, ok)

						require.Equal(t, "error", m["level"])
						require.Equal(t, msg, m["msg"])
						require.Equal(t, map[string]any{"service.name": "otelcol"}, m["resource"])

						if disableCaller {
							require.NotContains(t, m, "caller")
						} else {
							require.Regexp(t, `^\S+\.go:\d+$`, m["caller"])
						}
						if named {
							require.Equal(t, loggerName, m["logger"])
						} else {
							require.NotContains(t, m, "logger")
						}
						if stacktrace {
							require.Contains(t, m["stacktrace"], "otelcol.encodeWithZap")
						} else {
							require.NotContains(t, m, "stacktrace")
						}

						e := entry.New()
						e.Body = line
						require.NoError(t, newTestParser(t).Process(t.Context(), e))
						require.Equal(t, msg, e.Body)
						require.Equal(t, entry.Error, e.Severity)
						require.False(t, e.Timestamp.IsZero())
					})
				}
			}
		}
	}
}

// encodeWithZap mirrors how the collector builds its zap logger in
// service/telemetry/otelconftelemetry: the production encoder config with an
// ISO8601 clock, and the resource attributes attached as a dict field.
func encodeWithZap(t *testing.T, encoding string, disableCaller, named, stacktrace bool, msg string) string {
	t.Helper()

	ec := zap.NewProductionEncoderConfig()
	ec.EncodeTime = zapcore.ISO8601TimeEncoder

	var enc zapcore.Encoder
	if encoding == "console" {
		enc = zapcore.NewConsoleEncoder(ec)
	} else {
		enc = zapcore.NewJSONEncoder(ec)
	}

	buf := &bytes.Buffer{}
	var opts []zap.Option
	if !disableCaller {
		opts = append(opts, zap.AddCaller())
	}
	if stacktrace {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	logger := zap.New(zapcore.NewCore(enc, zapcore.AddSync(buf), zapcore.DebugLevel), opts...)
	logger = logger.With(zap.Dict("resource", zap.String("service.name", "otelcol")))
	if named {
		logger = logger.Named(loggerName)
	}
	logger.Error(msg)
	require.NoError(t, logger.Sync())

	return strings.TrimRight(buf.String(), "\n")
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
			expectedErrMsg: `value "not-a-timestamp" is not an ISO8601 timestamp`,
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

func TestSetSeverity(t *testing.T) {
	cases := []struct {
		level string
		want  entry.Severity
	}{
		{"debug", entry.Debug},
		{"info", entry.Info},
		{"warn", entry.Warn},
		{"error", entry.Error},
		{"dpanic", entry.Fatal},
		{"panic", entry.Fatal2},
		{"fatal", entry.Fatal3},
		{"unknown", entry.Default},
	}

	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			e := entry.New()
			m := map[string]any{"level": tc.level}
			setSeverity(e, m)
			require.Equal(t, tc.want, e.Severity)
			require.Equal(t, tc.level, e.SeverityText)
			require.NotContains(t, m, "level")
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
