// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/libhoneyevent"
)

func TestGetDatasetFromRequest(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty path",
			path:        "",
			want:        "",
			wantErr:     true,
			errContains: "missing dataset name",
		},
		{
			name: "simple path",
			path: "mydataset",
			want: "mydataset",
		},
		{
			name: "encoded path",
			path: "my%20dataset",
			want: "my dataset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetDatasetFromRequest(tt.path)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToPdata(t *testing.T) {
	logger := zap.NewNop()
	now := time.Now()

	// Create test trace and span IDs
	traceID := make([]byte, 16)
	spanID := make([]byte, 8)
	_, err := hex.Decode(traceID, []byte("1234567890abcdef1234567890abcdef"))
	require.NoError(t, err)
	_, err = hex.Decode(spanID, []byte("1234567890abcdef"))
	require.NoError(t, err)

	testCfg := libhoneyevent.FieldMapConfig{
		Attributes: libhoneyevent.AttributesConfig{
			Name:           "name",
			TraceID:        "trace.trace_id",
			SpanID:         "trace.span_id",
			ParentID:       "trace.parent_id",
			Error:          "error",
			SpanKind:       "kind",
			DurationFields: []string{"duration_ms"},
		},
		Resources: libhoneyevent.ResourcesConfig{
			ServiceName: "service.name",
		},
		Scopes: libhoneyevent.ScopesConfig{
			LibraryName:    "library.name",
			LibraryVersion: "library.version",
		},
	}

	tests := []struct {
		name      string
		dataset   string
		events    []libhoneyevent.LibhoneyEvent
		cfg       libhoneyevent.FieldMapConfig
		wantLogs  int
		wantSpans int
	}{
		{
			name:      "empty events",
			dataset:   "test-dataset",
			events:    []libhoneyevent.LibhoneyEvent{},
			cfg:       testCfg,
			wantLogs:  0,
			wantSpans: 0,
		},
		{
			name:    "single span",
			dataset: "test-dataset",
			events: []libhoneyevent.LibhoneyEvent{
				{
					Data: map[string]any{
						"meta.signal_type": "trace",
						"trace.trace_id":   hex.EncodeToString(traceID),
						"trace.span_id":    hex.EncodeToString(spanID),
						"name":             "test-span",
						"duration_ms":      100.1,
						"error":            true,
						"status_message":   "error message",
						"kind":             "server",
					},
					MsgPackTimestamp: &now,
					Samplerate:       1,
				},
			},
			cfg:       testCfg,
			wantLogs:  0,
			wantSpans: 1,
		},
		{
			name:    "single log",
			dataset: "test-dataset",
			events: []libhoneyevent.LibhoneyEvent{
				{
					Data: map[string]any{
						"meta.signal_type": "log",
						"message":          "test log message",
					},
					MsgPackTimestamp: &now,
				},
			},
			cfg:       testCfg,
			wantLogs:  1,
			wantSpans: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs, traces := ToPdata(tt.dataset, tt.events, tt.cfg, *logger)
			assert.Equal(t, tt.wantSpans, traces.SpanCount())
			assert.Equal(t, tt.wantLogs, logs.LogRecordCount())
		})
	}
}

// Helper function to verify attributes
func verifyAttributes(t *testing.T, expected, actual pcommon.Map) {
	for k, v := range expected.All() {
		got, ok := actual.Get(k)
		assert.True(t, ok, "missing attribute %s", k)
		assert.Equal(t, v.Type(), got.Type(), "wrong type for attribute %s", k)
		assert.Equal(t, v, got, "wrong value for attribute %s", k)
	}
}

func TestAddSpanEventsToSpan(t *testing.T) {
	logger := zap.NewNop()
	now := time.Now()
	s := ptrace.NewSpan()
	s.SetName("test-span")
	s.SetSpanID([8]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef})
	s.SetTraceID([16]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef})
	s.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	s.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(100 * time.Millisecond)))
	s.Status().SetCode(ptrace.StatusCodeError)
	s.Status().SetMessage("error message")
	s.SetKind(ptrace.SpanKindServer)
	s.Attributes().PutStr("string_attr", "value")
	s.Attributes().PutInt("int_attr", 42)
	s.Attributes().PutBool("bool_attr", true)

	events := []libhoneyevent.LibhoneyEvent{
		{
			Data: map[string]any{
				"name":   "event1",
				"string": "value",
				"int":    42,
				"float":  3.14,
				"bool":   true,
			},
			MsgPackTimestamp: &now,
		},
	}

	addSpanEventsToSpan(s, events, []string{}, logger)

	assert.Equal(t, 1, s.Events().Len())
	event := s.Events().At(0)
	assert.Equal(t, "event1", event.Name())
	assert.Equal(t, pcommon.Timestamp(now.UnixNano()), event.Timestamp())

	attrs := event.Attributes()
	verifyAttributes(t, attrs, event.Attributes())
}

func TestAddSpanLinksToSpan(t *testing.T) {
	logger := zap.NewNop()

	now := time.Now()
	s := ptrace.NewSpan()
	s.SetName("test-span")
	s.SetSpanID([8]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef})
	s.SetTraceID([16]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef})
	s.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	s.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(100 * time.Millisecond)))
	s.Status().SetCode(ptrace.StatusCodeError)
	s.Status().SetMessage("error message")
	s.SetKind(ptrace.SpanKindServer)
	s.Attributes().PutStr("string_attr", "value")
	s.Attributes().PutInt("int_attr", 42)
	s.Attributes().PutBool("bool_attr", true)

	// Create test trace and span IDs for the link
	linkTraceIDHex := make([]byte, 16)
	linkSpanIDHex := make([]byte, 8)
	_, err := hex.Decode(linkTraceIDHex, []byte("abcdef1234567890abcdef1234567890"))
	require.NoError(t, err)
	_, err = hex.Decode(linkSpanIDHex, []byte("abcdef1234567890"))
	require.NoError(t, err)
	linkTraceID := pcommon.TraceID(linkTraceIDHex)
	linkSpanID := pcommon.SpanID(linkSpanIDHex)

	links := []libhoneyevent.LibhoneyEvent{
		{
			Data: map[string]any{
				"trace.link.trace_id": hex.EncodeToString(linkTraceIDHex),
				"trace.link.span_id":  hex.EncodeToString(linkSpanIDHex),
				"trace.trace_id":      "1234567890abcdef1234567890abcdef",
				"trace.span_id":       "1234567890abcdef",
				"attribute1":          "stringval",
				"attribute2":          4,
			},
		},
	}

	addSpanLinksToSpan(s, links, []string{}, logger)

	assert.Equal(t, 1, s.Links().Len())
	link := s.Links().At(0)

	// Verify trace and span IDs
	assert.Equal(t, linkTraceID, link.TraceID())
	assert.Equal(t, linkSpanID, link.SpanID())

	// Verify attributes
	attrs := link.Attributes()
	verifyAttributes(t, attrs, link.Attributes())
}
