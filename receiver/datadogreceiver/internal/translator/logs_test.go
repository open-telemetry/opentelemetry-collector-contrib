// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

// receivedAt is a fixed request-received time so ObservedTimestamp is deterministic in tests.
var receivedAt = time.Unix(1700000000, 0).UTC()

// unmarshalLogs parses a JSON intake body the same way handleLogs does, exercising the custom
// DatadogLogPayload.UnmarshalJSON (which routes non-reserved fields into Additional).
func unmarshalLogs(t *testing.T, body string) []*DatadogLogPayload {
	t.Helper()
	var logs []*DatadogLogPayload
	require.NoError(t, json.Unmarshal([]byte(body), &logs))
	return logs
}

func TestToPlog(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		expectEmpty bool
	}{
		{
			name:        "empty logs array",
			body:        `[]`,
			expectEmpty: true,
		},
		{
			name: "single_log_entry",
			body: `[{
				"message": "Test log message",
				"status": "info",
				"timestamp": 1234567890,
				"hostname": "test-host",
				"service": "test-service",
				"ddsource": "test-source",
				"ddtags": "env:prod,version:1.0"
			}]`,
		},
		{
			name: "multiple_log_entries",
			body: `[
				{"message":"First log","status":"info","timestamp":1000000000,"hostname":"host1","service":"service1","ddsource":"source1","ddtags":"tag1:value1"},
				{"message":"Second log","status":"error","timestamp":2000000000,"hostname":"host2","service":"service2","ddsource":"source2","ddtags":"tag2:value2,tagB:valueB"},
				{"message":"Third log","status":"warning","timestamp":3000000000,"hostname":"host3","service":"service3","ddsource":"source3","ddtags":"tag3:value3"}
			]`,
		},
		{
			name: "log_with_trace_correlation",
			body: `[{
				"message": "correlated log",
				"status": "error",
				"timestamp": 1700000000000,
				"hostname": "h",
				"service": "svc",
				"ddsource": "python",
				"ddtags": "env:prod",
				"dd.trace_id": "1234567890123456789",
				"dd.span_id": "987654321",
				"dd.service": "svc-injected",
				"dd.env": "staging",
				"dd.version": "2.0.0",
				"error.msg": "boom",
				"custom.count": 42,
				"custom.flag": true
			}]`,
		},
		{
			name: "log_with_empty_strings",
			body: `[{"message":"","status":"","hostname":"","service":"","ddsource":"","ddtags":""}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ToPlog(unmarshalLogs(t, tt.body), receivedAt, false)

			if tt.expectEmpty {
				assert.Equal(t, 0, actual.ResourceLogs().Len())
				return
			}

			expectedFile := filepath.Join("testdata", "logs", tt.name+".yaml")
			// Regenerate golden files with: REGEN_GOLDEN=1 go test ./receiver/datadogreceiver/...
			if os.Getenv("REGEN_GOLDEN") != "" {
				require.NoError(t, golden.WriteLogs(t, expectedFile, actual))
			}
			expected, err := golden.ReadLogs(expectedFile)
			require.NoError(t, err)
			// Timestamps are intentionally NOT ignored: asserting them is the whole point of this fix.
			require.NoError(t, plogtest.CompareLogs(expected, actual,
				plogtest.IgnoreResourceLogsOrder(),
			))
		})
	}
}

func TestToPlogEmpty(t *testing.T) {
	assert.Equal(t, 0, ToPlog(nil, receivedAt, false).ResourceLogs().Len())
	assert.Equal(t, 0, ToPlog([]*DatadogLogPayload{}, receivedAt, false).ResourceLogs().Len())
}

// firstRecord returns the single log record produced for a one-item body.
func firstRecord(t *testing.T, body string) plog.LogRecord {
	t.Helper()
	logs := ToPlog(unmarshalLogs(t, body), receivedAt, false)
	require.Equal(t, 1, logs.ResourceLogs().Len())
	rl := logs.ResourceLogs().At(0)
	require.Equal(t, 1, rl.ScopeLogs().Len())
	sl := rl.ScopeLogs().At(0)
	require.Equal(t, 1, sl.LogRecords().Len())
	return sl.LogRecords().At(0)
}

func TestToPlogTimestamps(t *testing.T) {
	t.Run("epoch millis converted to nanos", func(t *testing.T) {
		lr := firstRecord(t, `[{"message":"m","timestamp":1700000000000}]`)
		// 1700000000000 ms * 1e6 = 1700000000000000000 ns
		assert.Equal(t, uint64(1700000000000000000), uint64(lr.Timestamp()))
		assert.Equal(t, uint64(1700000000000000000), uint64(lr.ObservedTimestamp()))
	})

	t.Run("missing timestamp leaves Timestamp unset but sets ObservedTimestamp", func(t *testing.T) {
		lr := firstRecord(t, `[{"message":"no ts"}]`)
		assert.Equal(t, uint64(0), uint64(lr.Timestamp()))
		assert.Equal(t, uint64(1700000000000000000), uint64(lr.ObservedTimestamp()))
	})

	t.Run("ISO8601 string timestamp parsed", func(t *testing.T) {
		lr := firstRecord(t, `[{"message":"m","timestamp":"2023-11-14T22:13:20Z"}]`)
		assert.Equal(t, time.Date(2023, 11, 14, 22, 13, 20, 0, time.UTC).UnixNano(), lr.Timestamp().AsTime().UnixNano())
	})

	t.Run("alternate @timestamp key", func(t *testing.T) {
		lr := firstRecord(t, `[{"message":"m","@timestamp":1700000000000}]`)
		assert.Equal(t, uint64(1700000000000000000), uint64(lr.Timestamp()))
	})
}

func TestStatusToSeverityNumber(t *testing.T) {
	cases := map[string]plog.SeverityNumber{
		"trace":     plog.SeverityNumberTrace,
		"debug":     plog.SeverityNumberDebug,
		"info":      plog.SeverityNumberInfo,
		"ok":        plog.SeverityNumberInfo,
		"notice":    plog.SeverityNumberInfo2,
		"warning":   plog.SeverityNumberWarn,
		"warn":      plog.SeverityNumberWarn,
		"error":     plog.SeverityNumberError,
		"critical":  plog.SeverityNumberError2,
		"alert":     plog.SeverityNumberError3,
		"emergency": plog.SeverityNumberFatal,
		"ERROR":     plog.SeverityNumberError, // case-insensitive
		"":          plog.SeverityNumberUnspecified,
		"bogus":     plog.SeverityNumberUnspecified,
	}
	for status, want := range cases {
		assert.Equalf(t, want, statusToSeverityNumber(status), "status %q", status)
	}
}

func TestToPlogSeverityTextPreserved(t *testing.T) {
	lr := firstRecord(t, `[{"message":"m","status":"WARNING"}]`)
	assert.Equal(t, "WARNING", lr.SeverityText())
	assert.Equal(t, plog.SeverityNumberWarn, lr.SeverityNumber())
}

func TestToPlogTraceCorrelation(t *testing.T) {
	lr := firstRecord(t, `[{
		"message":"m",
		"dd.trace_id":"1234567890123456789",
		"dd.span_id":"987654321"
	}]`)
	assert.False(t, lr.TraceID().IsEmpty())
	assert.False(t, lr.SpanID().IsEmpty())
	assert.Equal(t, uInt64ToTraceID(0, 1234567890123456789), lr.TraceID())
	assert.Equal(t, uInt64ToSpanID(987654321), lr.SpanID())
	// Correlation keys must not also leak into raw attributes.
	_, ok := lr.Attributes().Get("dd.trace_id")
	assert.False(t, ok)
}

func TestToPlogTraceID128Bit(t *testing.T) {
	t.Run("64-bit decimal zero-pads the high bits", func(t *testing.T) {
		lr := firstRecord(t, `[{"message":"m","dd.trace_id":"1234567890123456789"}]`)
		assert.Equal(t, uInt64ToTraceID(0, 1234567890123456789), lr.TraceID())
	})

	t.Run("_dd.p.tid reconstructs the full 128-bit id", func(t *testing.T) {
		// Upper 64 bits "640cfd1e00000000" (hex) + lower 64 bits 1234567890123456789.
		lr := firstRecord(t, `[{"message":"m","dd.trace_id":"1234567890123456789","_dd.p.tid":"640cfd1e00000000"}]`)
		upper, err := strconv.ParseUint("640cfd1e00000000", 16, 64)
		require.NoError(t, err)
		assert.Equal(t, uInt64ToTraceID(upper, 1234567890123456789), lr.TraceID())
		// The reconstruction tag must not also leak as an attribute.
		_, ok := lr.Attributes().Get("_dd.p.tid")
		assert.False(t, ok)
	})

	t.Run("32-char hex is taken as a full 128-bit id (OTel-origin)", func(t *testing.T) {
		lr := firstRecord(t, `[{"message":"m","dd.trace_id":"f3c18530c08e00a43b881a9ce0197d39","dd.span_id":"3b881a9ce0197d39"}]`)
		var want pcommon.TraceID
		b, err := hex.DecodeString("f3c18530c08e00a43b881a9ce0197d39")
		require.NoError(t, err)
		copy(want[:], b)
		assert.Equal(t, want, lr.TraceID())
		assert.False(t, lr.SpanID().IsEmpty())
	})

	t.Run("128-bit decimal id", func(t *testing.T) {
		// Same 128-bit value as the hex case above, rendered in decimal.
		var want pcommon.TraceID
		b, _ := hex.DecodeString("f3c18530c08e00a43b881a9ce0197d39")
		copy(want[:], b)
		dec := new(big.Int).SetBytes(want[:]).String()
		lr := firstRecord(t, `[{"message":"m","dd.trace_id":"`+dec+`"}]`)
		assert.Equal(t, want, lr.TraceID())
	})
}

func TestToPlogReservedDDResourceAttributes(t *testing.T) {
	logs := ToPlog(unmarshalLogs(t, `[{
		"message":"m",
		"service":"svc-from-field",
		"ddtags":"env:prod,version:1.0",
		"dd.service":"svc-injected",
		"dd.env":"staging",
		"dd.version":"2.0.0"
	}]`), receivedAt, false)
	res := logs.ResourceLogs().At(0).Resource().Attributes()
	// dd.* fields take precedence over the ddtags-derived and top-level service values.
	v, _ := res.Get("service.name")
	assert.Equal(t, "svc-injected", v.AsString())
	v, _ = res.Get("deployment.environment.name")
	assert.Equal(t, "staging", v.AsString())
	v, _ = res.Get("service.version")
	assert.Equal(t, "2.0.0", v.AsString())
}

func TestToPlogAdditionalAttributes(t *testing.T) {
	lr := firstRecord(t, `[{
		"message":"m",
		"error.msg":"boom",
		"custom.count":42,
		"custom.ratio":1.5,
		"custom.flag":true
	}]`)
	attrs := lr.Attributes()
	// error.msg is a known Datadog key translated to the OTel semantic convention.
	v, ok := attrs.Get("exception.message")
	require.True(t, ok)
	assert.Equal(t, "boom", v.AsString())
	v, ok = attrs.Get("custom.count")
	require.True(t, ok)
	assert.Equal(t, int64(42), v.Int())
	v, ok = attrs.Get("custom.ratio")
	require.True(t, ok)
	assert.Equal(t, 1.5, v.Double())
	v, ok = attrs.Get("custom.flag")
	require.True(t, ok)
	assert.True(t, v.Bool())
}

func TestToPlogDecodeJSONMessage(t *testing.T) {
	// The agent forwards an application JSON log as an opaque message string, defaults status to
	// "info", stamps its own collection time, and adds container tags. With decodeJSONMessage on, the
	// inner reserved fields must win and dd.trace_id/dd.span_id must populate the real TraceID/SpanID.
	envelope := `[{
		"message": "{\"message\":\"GET /login 429\",\"status\":\"warn\",\"timestamp\":1700000000000,\"dd.trace_id\":\"3496233055802637027\",\"dd.span_id\":\"7255197583904306129\",\"dd.env\":\"prod\",\"http.method\":\"GET\"}",
		"status": "info",
		"hostname": "dd-demo-host",
		"service": "checkout",
		"ddsource": "go-demo",
		"ddtags": "container_id:abc123"
	}]`

	in := unmarshalLogs(t, envelope)

	// Without decoding: body stays the raw JSON, status is the agent default, no correlation.
	off := ToPlog(in, receivedAt, false)
	rec := off.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Contains(t, rec.Body().Str(), `"dd.trace_id"`)
	assert.Equal(t, "info", rec.SeverityText())
	assert.True(t, rec.TraceID().IsEmpty())

	// With decoding: inner fields win and correlation is populated.
	on := ToPlog(unmarshalLogs(t, envelope), receivedAt, true)
	require.Equal(t, 1, on.ResourceLogs().Len())
	rl := on.ResourceLogs().At(0)
	rec = rl.ScopeLogs().At(0).LogRecords().At(0)

	assert.Equal(t, "GET /login 429", rec.Body().Str())
	assert.Equal(t, "warn", rec.SeverityText())
	assert.Equal(t, plog.SeverityNumberWarn, rec.SeverityNumber())
	assert.Equal(t, uInt64ToTraceID(0, 3496233055802637027), rec.TraceID())
	assert.Equal(t, uInt64ToSpanID(7255197583904306129), rec.SpanID())
	// Inner app timestamp (1700000000000 ms) wins over the agent's collection time.
	assert.Equal(t, uint64(1700000000000000000), uint64(rec.Timestamp()))

	res := rl.Resource().Attributes().AsRaw()
	assert.Equal(t, "prod", res["deployment.environment.name"]) // from inner dd.env
	assert.Equal(t, "checkout", res["service.name"])            // from envelope
	assert.Equal(t, "abc123", res["container.id"])              // from envelope ddtags

	attrs := rec.Attributes().AsRaw()
	// inner non-reserved key -> attribute, with Datadog->OTel key translation (http.method -> http.request.method)
	assert.Equal(t, "GET", attrs["http.request.method"])
	assert.Equal(t, "go-demo", attrs["datadog.ddsource"])
}

func TestToPlogGroupsByResource(t *testing.T) {
	// Two logs from the same host/service group into one ResourceLogs; a third differs.
	logs := ToPlog(unmarshalLogs(t, `[
		{"message":"a","hostname":"h1","service":"s1"},
		{"message":"b","hostname":"h1","service":"s1"},
		{"message":"c","hostname":"h2","service":"s2"}
	]`), receivedAt, false)
	assert.Equal(t, 2, logs.ResourceLogs().Len())
}
