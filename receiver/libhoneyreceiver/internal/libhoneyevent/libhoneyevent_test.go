// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyevent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	trc "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func TestLibhoneyEvent_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    LibhoneyEvent
		wantErr bool
	}{
		{
			name: "basic event",
			json: `{
				"time": "2024-01-01T00:00:00Z",
				"data": {"key": "value"},
				"samplerate": 1
			}`,
			want: LibhoneyEvent{
				Time:       "2024-01-01T00:00:00Z",
				Data:       map[string]any{"key": "value"},
				Samplerate: 1,
			},
		},
		{
			name:    "invalid json",
			json:    `{invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got LibhoneyEvent
			err := json.Unmarshal([]byte(tt.json), &got)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want.Time, got.Time)
			assert.Equal(t, tt.want.Data, got.Data)
			assert.Equal(t, tt.want.Samplerate, got.Samplerate)
			assert.NotNil(t, got.MsgPackTimestamp)
		})
	}

	test := struct {
		name    string
		json    string
		want    LibhoneyEvent
		wantErr bool
	}{
		name: "missing time uses current",
		json: `{
			"data": {"key": "value"},
			"samplerate": 2
		}`,
		want: LibhoneyEvent{
			Time:       "",
			Data:       map[string]any{"key": "value"},
			Samplerate: 2,
		},
	}
	t.Run(test.name, func(t *testing.T) {
		var got LibhoneyEvent
		err := json.Unmarshal([]byte(test.json), &got)

		require.NoError(t, err)
		assert.Equal(t, test.want.Data, got.Data)
		gotTime, timeErr := time.Parse(time.RFC3339Nano, got.Time)
		assert.NoError(t, timeErr)
		assert.WithinDuration(t, time.Now(), gotTime, time.Second)
		assert.Equal(t, test.want.Samplerate, got.Samplerate)
		assert.NotNil(t, got.MsgPackTimestamp)
	})
}

func TestLibHoneyEvent_ToPLogRecord(t *testing.T) {
	logger := zap.NewNop()
	now := time.Now()
	tests := []struct {
		name              string
		event             LibhoneyEvent
		alreadyUsedFields []string
		want              func(plog.LogRecord)
		wantErr           bool
	}{
		{
			name: "basic conversion",
			event: LibhoneyEvent{
				Samplerate:       1,
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"severity_text": "ERROR",
					"severity_code": int64(2),
					"body":          "test message",
					"string_attr":   "value",
					"int_attr":      42,
					"float_attr":    3.14,
					"bool_attr":     true,
				},
			},
			want: func(lr plog.LogRecord) {
				lr.SetSeverityText("ERROR")
				lr.SetSeverityNumber(plog.SeverityNumber(2))
				lr.Body().SetStr("test message")
				lr.Attributes().PutStr("string_attr", "value")
				lr.Attributes().PutInt("int_attr", 42)
				lr.Attributes().PutDouble("float_attr", 3.14)
				lr.Attributes().PutBool("bool_attr", true)
				lr.Attributes().PutInt("SampleRate", 1)
			},
		},
		{
			name: "skip already used fields",
			event: LibhoneyEvent{
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"skip_me": "value",
					"keep_me": "value",
				},
			},
			alreadyUsedFields: []string{"skip_me"},
			want: func(lr plog.LogRecord) {
				lr.Attributes().PutStr("keep_me", "value")
				lr.Attributes().PutInt("SampleRate", 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newLog := plog.NewLogRecord()
			err := tt.event.ToPLogRecord(&newLog, &tt.alreadyUsedFields, *logger)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.want != nil {
				want := plog.NewLogRecord()
				tt.want(want)

				// Check severity
				assert.Equal(t, want.SeverityText(), newLog.SeverityText())
				assert.Equal(t, want.SeverityNumber(), newLog.SeverityNumber())

				// Check body
				assert.Equal(t, want.Body().AsString(), newLog.Body().AsString())

				// Check each attribute has correct type and value
				for k, v := range want.Attributes().All() {
					got, ok := newLog.Attributes().Get(k)
					assert.True(t, ok, "missing attribute %s", k)
					assert.Equal(t, v.Type(), got.Type(), "wrong type for attribute %s", k)
					assert.Equal(t, v, got, "wrong value for attribute %s", k)
				}

				// Verify no extra attributes
				assert.Equal(t, want.Attributes().Len(), newLog.Attributes().Len())
			}
		})
	}
}

func TestLibHoneyEvent_GetService(t *testing.T) {
	tests := []struct {
		name    string
		event   LibhoneyEvent
		fields  FieldMapConfig
		dataset string
		want    string
		wantErr bool
	}{
		{
			name: "service name found",
			event: LibhoneyEvent{
				Data: map[string]any{
					"service.name": "test-service",
				},
			},
			fields: FieldMapConfig{
				Resources: ResourcesConfig{
					ServiceName: "service.name",
				},
			},
			want: "test-service",
		},
		{
			name: "service name not found",
			event: LibhoneyEvent{
				Data: map[string]any{},
			},
			fields: FieldMapConfig{
				Resources: ResourcesConfig{
					ServiceName: "service.name",
				},
			},
			dataset: "default-dataset",
			want:    "default-dataset",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := &ServiceHistory{NameCount: make(map[string]int)}
			got, err := tt.event.GetService(tt.fields, seen, tt.dataset)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLibhoneyEvent_GetScope(t *testing.T) {
	tests := []struct {
		name        string
		event       LibhoneyEvent
		fields      FieldMapConfig
		serviceName string
		want        string
		wantErr     bool
	}{
		{
			name: "scope found",
			event: LibhoneyEvent{
				Data: map[string]any{
					"library.name":    "test-lib",
					"library.version": "1.0.0",
				},
			},
			fields: FieldMapConfig{
				Scopes: ScopesConfig{
					LibraryName:    "library.name",
					LibraryVersion: "library.version",
				},
			},
			serviceName: "test-service",
			want:        "test-servicetest-lib",
		},
		{
			name: "scope not found",
			event: LibhoneyEvent{
				Data: map[string]any{},
			},
			fields: FieldMapConfig{
				Scopes: ScopesConfig{
					LibraryName: "library.name",
				},
			},
			serviceName: "test-service",
			want:        "test-service-libhoney.receiver",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := &ScopeHistory{Scope: make(map[string]SimpleScope)}
			got, err := tt.event.GetScope(tt.fields, seen, tt.serviceName)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToPTraceSpan(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		event   LibhoneyEvent
		want    func(ptrace.Span)
		wantErr bool
	}{
		{
			name: "basic span conversion",
			event: LibhoneyEvent{
				Time:             now.Format(time.RFC3339),
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"name":           "test-span",
					"trace.span_id":  "1234567890abcdef",
					"trace.trace_id": "1234567890abcdef1234567890abcdef",
					"duration_ms":    100.0,
					"error":          true,
					"status_message": "error message",
					"kind":           "server",
					"string_attr":    "value",
					"int_attr":       42,
					"bool_attr":      true,
				},
				Samplerate: 1,
			},
			want: func(s ptrace.Span) {
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
			},
		},
		{
			name: "valid hex parent ID is correctly parsed",
			event: LibhoneyEvent{
				Time:             now.Format(time.RFC3339),
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"name":            "child-span",
					"trace.span_id":   "abcdef1234567890",
					"trace.trace_id":  "1234567890abcdef1234567890abcdef",
					"trace.parent_id": "1234567890abcdef", // Valid hex-encoded span ID
					"duration_ms":     50.0,
				},
				Samplerate: 1,
			},
			want: func(s ptrace.Span) {
				s.SetName("child-span")
				s.SetSpanID([8]byte{0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90})
				s.SetTraceID([16]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef})
				// Parent ID should be the actual hex-decoded value, not a hash
				s.SetParentSpanID([8]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef})
				s.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
				s.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(50 * time.Millisecond)))
			},
		},
		{
			name: "invalid parent ID falls back to hash",
			event: LibhoneyEvent{
				Time:             now.Format(time.RFC3339),
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"name":            "child-span",
					"trace.span_id":   "abcdef1234567890",
					"trace.trace_id":  "1234567890abcdef1234567890abcdef",
					"trace.parent_id": "invalid-hex-string", // Invalid hex, should fall back to hash
					"duration_ms":     50.0,
				},
				Samplerate: 1,
			},
			want: func(s ptrace.Span) {
				s.SetName("child-span")
				s.SetSpanID([8]byte{0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90})
				s.SetTraceID([16]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef})
				// Parent ID should be the hash of "invalid-hex-string"
				expectedHash := spanIDFrom("invalid-hex-string")
				s.SetParentSpanID(pcommon.SpanID(expectedHash))
				s.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
				s.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(50 * time.Millisecond)))
			},
		},
		{
			name: "sub-1-millisecond duration",
			event: LibhoneyEvent{
				Time:             now.Format(time.RFC3339),
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"name":           "fast-span",
					"trace.span_id":  "1234567890abcdef",
					"trace.trace_id": "1234567890abcdef1234567890abcdef",
					"duration_ms":    0.5, // 500 microseconds
				},
				Samplerate: 1,
			},
			want: func(s ptrace.Span) {
				s.SetName("fast-span")
				s.SetSpanID([8]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef})
				s.SetTraceID([16]byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef})
				s.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
				s.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(500 * time.Microsecond)))
			},
		},
	}

	alreadyUsedFields := []string{"name", "trace.span_id", "trace.trace_id", "duration_ms", "status.code", "status.message", "kind"}
	testCfg := FieldMapConfig{
		Attributes: AttributesConfig{
			Name:           "name",
			TraceID:        "trace.trace_id",
			SpanID:         "trace.span_id",
			ParentID:       "trace.parent_id",
			Error:          "error",
			SpanKind:       "kind",
			DurationFields: []string{"duration_ms"},
		},
		Resources: ResourcesConfig{
			ServiceName: "service.name",
		},
		Scopes: ScopesConfig{
			LibraryName:    "library.name",
			LibraryVersion: "library.version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			err := tt.event.ToPTraceSpan(&span, &alreadyUsedFields, testCfg, *zap.NewNop())

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.want != nil {
				want := ptrace.NewSpan()
				tt.want(want)

				// Check basic fields
				assert.Equal(t, want.Name(), span.Name())
				assert.Equal(t, want.SpanID(), span.SpanID())
				assert.Equal(t, want.TraceID(), span.TraceID())
				assert.Equal(t, want.StartTimestamp(), span.StartTimestamp())
				assert.Equal(t, want.EndTimestamp(), span.EndTimestamp())
				assert.Equal(t, want.Kind(), span.Kind())

				// Check status
				assert.Equal(t, want.Status().Code(), span.Status().Code())
				assert.Equal(t, want.Status().Message(), span.Status().Message())

				// Check attributes
				for k, v := range want.Attributes().All() {
					got, ok := span.Attributes().Get(k)
					assert.True(t, ok, "missing attribute %s", k)
					assert.Equal(t, v.Type(), got.Type(), "wrong type for attribute %s", k)
					assert.Equal(t, v, got, "wrong value for attribute %s", k)
				}

				// Verify no fewer attributes, extras are expected
				assert.LessOrEqual(t, want.Attributes().Len(), span.Attributes().Len())
			}
		})
	}
}

func TestParentIDHandlingDifference(t *testing.T) {
	validHexParentID := "1234567890abcdef"

	oldApproachResult := spanIDFrom(validHexParentID)

	// What the new approach produces (parsing hex)
	event := LibhoneyEvent{
		Data: map[string]any{
			"trace.parent_id": validHexParentID,
		},
	}
	newApproachResult, err := event.GetParentID("trace.parent_id")
	require.NoError(t, err)

	assert.NotEqual(t, oldApproachResult, newApproachResult,
		"Old and new approaches should produce different results for valid hex parent ID")

	expectedBytes := []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef}
	assert.Equal(t, trc.SpanID(expectedBytes), newApproachResult,
		"New approach should correctly decode hex parent ID")

	// For invalid hex, both approaches should produce the same result (hash)
	invalidParentID := "invalid-hex-string"
	oldInvalidResult := spanIDFrom(invalidParentID)

	eventInvalid := LibhoneyEvent{
		Data: map[string]any{
			"trace.parent_id": invalidParentID,
		},
	}
	_, err = eventInvalid.GetParentID("trace.parent_id")
	assert.Error(t, err, "GetParentID should return error for invalid hex")

	// The ToPTraceSpan function should fall back to hash for invalid hex
	span := ptrace.NewSpan()
	cfg := FieldMapConfig{
		Attributes: AttributesConfig{
			ParentID: "trace.parent_id",
		},
	}

	eventInvalid.MsgPackTimestamp = &time.Time{}
	err = eventInvalid.ToPTraceSpan(&span, &[]string{}, cfg, *zap.NewNop())
	require.NoError(t, err)

	// Should fall back to hash for invalid hex
	assert.Equal(t, pcommon.SpanID(oldInvalidResult), span.ParentSpanID(),
		"Invalid hex should fall back to hash in new approach")
}

func TestGetParentID(t *testing.T) {
	tests := []struct {
		name      string
		event     LibhoneyEvent
		fieldName string
		want      trc.SpanID
		wantErr   bool
	}{
		{
			name: "valid 16-character hex string",
			event: LibhoneyEvent{
				Data: map[string]any{
					"trace.parent_id": "1234567890abcdef",
				},
			},
			fieldName: "trace.parent_id",
			want:      trc.SpanID{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
		},
		{
			name: "valid 32-character hex string (trace ID format)",
			event: LibhoneyEvent{
				Data: map[string]any{
					"trace.parent_id": "1234567890abcdef1234567890abcdef",
				},
			},
			fieldName: "trace.parent_id",
			want:      trc.SpanID{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
		},
		{
			name: "hex string with hyphens",
			event: LibhoneyEvent{
				Data: map[string]any{
					"trace.parent_id": "12-34-56-78-90-ab-cd-ef",
				},
			},
			fieldName: "trace.parent_id",
			want:      trc.SpanID{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
		},
		{
			name: "invalid hex string",
			event: LibhoneyEvent{
				Data: map[string]any{
					"trace.parent_id": "invalid-hex",
				},
			},
			fieldName: "trace.parent_id",
			wantErr:   true,
		},
		{
			name: "field not found",
			event: LibhoneyEvent{
				Data: map[string]any{},
			},
			fieldName: "trace.parent_id",
			wantErr:   true,
		},
		{
			name: "odd length hex string",
			event: LibhoneyEvent{
				Data: map[string]any{
					"trace.parent_id": "1234567890abcde", // 15 characters
				},
			},
			fieldName: "trace.parent_id",
			wantErr:   true,
		},
		{
			name: "empty string",
			event: LibhoneyEvent{
				Data: map[string]any{
					"trace.parent_id": "",
				},
			},
			fieldName: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.event.GetParentID(tt.fieldName)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToPLogRecord_IntegerTypeHandling(t *testing.T) {
	logger := zap.NewNop()
	now := time.Now()

	tests := []struct {
		name              string
		event             LibhoneyEvent
		alreadyUsedFields []string
		checkFunc         func(t *testing.T, lr plog.LogRecord)
	}{
		{
			name: "severity_code as int64",
			event: LibhoneyEvent{
				Samplerate:       1,
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"severity_code": int64(5),
				},
			},
			checkFunc: func(t *testing.T, lr plog.LogRecord) {
				assert.Equal(t, plog.SeverityNumber(5), lr.SeverityNumber())
			},
		},
		{
			name: "severity_code as uint64",
			event: LibhoneyEvent{
				Samplerate:       1,
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"severity_code": uint64(5),
				},
			},
			checkFunc: func(t *testing.T, lr plog.LogRecord) {
				assert.Equal(t, plog.SeverityNumber(5), lr.SeverityNumber())
			},
		},
		{
			name: "flags as int64",
			event: LibhoneyEvent{
				Samplerate:       1,
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"flags": int64(1),
				},
			},
			checkFunc: func(t *testing.T, lr plog.LogRecord) {
				assert.Equal(t, plog.LogRecordFlags(1), lr.Flags())
			},
		},
		{
			name: "flags as uint64",
			event: LibhoneyEvent{
				Samplerate:       1,
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"flags": uint64(1),
				},
			},
			checkFunc: func(t *testing.T, lr plog.LogRecord) {
				assert.Equal(t, plog.LogRecordFlags(1), lr.Flags())
			},
		},
		{
			name: "attribute as int64",
			event: LibhoneyEvent{
				Samplerate:       1,
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"my_int_attr": int64(42),
				},
			},
			checkFunc: func(t *testing.T, lr plog.LogRecord) {
				val, ok := lr.Attributes().Get("my_int_attr")
				assert.True(t, ok)
				assert.Equal(t, int64(42), val.Int())
			},
		},
		{
			name: "attribute as uint64",
			event: LibhoneyEvent{
				Samplerate:       1,
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"my_uint_attr": uint64(42),
				},
			},
			checkFunc: func(t *testing.T, lr plog.LogRecord) {
				val, ok := lr.Attributes().Get("my_uint_attr")
				assert.True(t, ok)
				assert.Equal(t, int64(42), val.Int())
			},
		},
		{
			name: "large uint64 attribute",
			event: LibhoneyEvent{
				Samplerate:       1,
				MsgPackTimestamp: &now,
				Data: map[string]any{
					"large_uint": uint64(18446744073709551615), // max uint64
				},
			},
			checkFunc: func(t *testing.T, lr plog.LogRecord) {
				val, ok := lr.Attributes().Get("large_uint")
				assert.True(t, ok)
				// When converted to int64, this will overflow but should not panic
				assert.Equal(t, int64(-1), val.Int())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newLog := plog.NewLogRecord()
			err := tt.event.ToPLogRecord(&newLog, &tt.alreadyUsedFields, *logger)
			require.NoError(t, err)
			tt.checkFunc(t, newLog)
		})
	}
}

func TestToPTraceSpan_IntegerTypeHandling(t *testing.T) {
	now := time.Now()
	alreadyUsedFields := []string{"name", "trace.span_id", "trace.trace_id", "duration_ms"}
	testCfg := FieldMapConfig{
		Attributes: AttributesConfig{
			Name:           "name",
			TraceID:        "trace.trace_id",
			SpanID:         "trace.span_id",
			ParentID:       "trace.parent_id",
			Error:          "error",
			SpanKind:       "kind",
			DurationFields: []string{"duration_ms"},
		},
	}

	tests := []struct {
		name      string
		event     LibhoneyEvent
		checkFunc func(t *testing.T, span ptrace.Span)
	}{
		{
			name: "attribute as int64",
			event: LibhoneyEvent{
				Time:             now.Format(time.RFC3339),
				MsgPackTimestamp: &now,
				Samplerate:       1,
				Data: map[string]any{
					"name":           "test-span",
					"trace.span_id":  "1234567890abcdef",
					"trace.trace_id": "1234567890abcdef1234567890abcdef",
					"duration_ms":    100.0,
					"my_int_attr":    int64(42),
				},
			},
			checkFunc: func(t *testing.T, span ptrace.Span) {
				val, ok := span.Attributes().Get("my_int_attr")
				assert.True(t, ok)
				assert.Equal(t, int64(42), val.Int())
			},
		},
		{
			name: "attribute as uint64",
			event: LibhoneyEvent{
				Time:             now.Format(time.RFC3339),
				MsgPackTimestamp: &now,
				Samplerate:       1,
				Data: map[string]any{
					"name":           "test-span",
					"trace.span_id":  "1234567890abcdef",
					"trace.trace_id": "1234567890abcdef1234567890abcdef",
					"duration_ms":    100.0,
					"my_uint_attr":   uint64(42),
				},
			},
			checkFunc: func(t *testing.T, span ptrace.Span) {
				val, ok := span.Attributes().Get("my_uint_attr")
				assert.True(t, ok)
				assert.Equal(t, int64(42), val.Int())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			err := tt.event.ToPTraceSpan(&span, &alreadyUsedFields, testCfg, *zap.NewNop())
			require.NoError(t, err)
			tt.checkFunc(t, span)
		})
	}
}

// TestToPTraceSpan_TimestampPreservation validates that when multiple spans in a batch
// have different timestamps, the resulting OTLP spans preserve the distinct start/end times.
// This test reproduces the issue where spans sent through Refinery → libhoneyreceiver
// end up with identical start timestamps (all spans start at the same time).
func TestToPTraceSpan_TimestampPreservation(t *testing.T) {
	alreadyUsedFields := []string{"name", "trace.span_id", "trace.trace_id", "trace.parent_id", "duration_ms"}
	testCfg := FieldMapConfig{
		Attributes: AttributesConfig{
			Name:           "name",
			TraceID:        "trace.trace_id",
			SpanID:         "trace.span_id",
			ParentID:       "trace.parent_id",
			Error:          "error",
			SpanKind:       "kind",
			DurationFields: []string{"duration_ms"},
		},
		Resources: ResourcesConfig{
			ServiceName: "service.name",
		},
		Scopes: ScopesConfig{
			LibraryName:    "library.name",
			LibraryVersion: "library.version",
		},
	}

	// These timestamps mirror the user's script: START_TIME=1769767008
	// Span 1: start=T, end=T+1s (duration=1000ms)
	// Span 2: start=T+1, end=T+3s (duration=2000ms)
	// Span 3: start=T+3, end=T+4s (duration=1000ms)
	// Root:   start=T, end=T+4s (duration=4000ms)
	baseTime := time.Unix(1769767008, 0).UTC()
	span1Time := baseTime
	span2Time := baseTime.Add(1 * time.Second)
	span3Time := baseTime.Add(3 * time.Second)
	rootTime := baseTime

	traceID := "854b46f148a643a2ed4075f240556292"
	rootSpanID := "b8d1d0affba3d6c2"

	t.Run("events with RFC3339 time strings preserve distinct timestamps", func(t *testing.T) {
		events := []LibhoneyEvent{
			{
				Time:             span1Time.Format(time.RFC3339Nano),
				MsgPackTimestamp: &span1Time,
				Samplerate:       1,
				Data: map[string]any{
					"name":            "test-span",
					"trace.trace_id":  traceID,
					"trace.span_id":   "aaaaaaaaaaaaaaaa",
					"trace.parent_id": rootSpanID,
					"duration_ms":     1000.0,
					"service.name":    "demo-svc",
				},
			},
			{
				Time:             span2Time.Format(time.RFC3339Nano),
				MsgPackTimestamp: &span2Time,
				Samplerate:       1,
				Data: map[string]any{
					"name":            "test-span",
					"trace.trace_id":  traceID,
					"trace.span_id":   "bbbbbbbbbbbbbbbb",
					"trace.parent_id": rootSpanID,
					"duration_ms":     2000.0,
					"service.name":    "demo-svc",
				},
			},
			{
				Time:             span3Time.Format(time.RFC3339Nano),
				MsgPackTimestamp: &span3Time,
				Samplerate:       1,
				Data: map[string]any{
					"name":            "test-span",
					"trace.trace_id":  traceID,
					"trace.span_id":   "cccccccccccccccc",
					"trace.parent_id": rootSpanID,
					"duration_ms":     1000.0,
					"service.name":    "demo-svc",
				},
			},
			{
				Time:             rootTime.Format(time.RFC3339Nano),
				MsgPackTimestamp: &rootTime,
				Samplerate:       1,
				Data: map[string]any{
					"name":           "test-span",
					"trace.trace_id": traceID,
					"trace.span_id":  rootSpanID,
					"duration_ms":    4000.0,
					"service.name":   "demo-svc",
				},
			},
		}

		spans := make([]ptrace.Span, len(events))
		for i, event := range events {
			spans[i] = ptrace.NewSpan()
			err := event.ToPTraceSpan(&spans[i], &alreadyUsedFields, testCfg, *zap.NewNop())
			require.NoError(t, err)
		}

		// Verify spans have DIFFERENT start timestamps (the core issue)
		assert.NotEqual(t, spans[0].StartTimestamp(), spans[1].StartTimestamp(),
			"span 1 and span 2 should have different start timestamps")
		assert.NotEqual(t, spans[1].StartTimestamp(), spans[2].StartTimestamp(),
			"span 2 and span 3 should have different start timestamps")

		// Verify exact timestamps
		assert.Equal(t, pcommon.NewTimestampFromTime(span1Time), spans[0].StartTimestamp(),
			"span 1 start should be T")
		assert.Equal(t, pcommon.NewTimestampFromTime(span1Time.Add(1*time.Second)), spans[0].EndTimestamp(),
			"span 1 end should be T+1s")

		assert.Equal(t, pcommon.NewTimestampFromTime(span2Time), spans[1].StartTimestamp(),
			"span 2 start should be T+1s")
		assert.Equal(t, pcommon.NewTimestampFromTime(span2Time.Add(2*time.Second)), spans[1].EndTimestamp(),
			"span 2 end should be T+3s")

		assert.Equal(t, pcommon.NewTimestampFromTime(span3Time), spans[2].StartTimestamp(),
			"span 3 start should be T+3s")
		assert.Equal(t, pcommon.NewTimestampFromTime(span3Time.Add(1*time.Second)), spans[2].EndTimestamp(),
			"span 3 end should be T+4s")

		assert.Equal(t, pcommon.NewTimestampFromTime(rootTime), spans[3].StartTimestamp(),
			"root span start should be T")
		assert.Equal(t, pcommon.NewTimestampFromTime(rootTime.Add(4*time.Second)), spans[3].EndTimestamp(),
			"root span end should be T+4s")
	})

	t.Run("JSON batch events without time field get time.Now and lose timestamp info", func(t *testing.T) {
		// This test demonstrates the bug: when events arrive without a "time" field
		// (or with an unparseable time), all events in the batch get time.Now(),
		// causing all spans to start at approximately the same time.
		jsonBatch := `[
			{"samplerate": 1, "data": {"name": "test-span", "trace.trace_id": "854b46f148a643a2ed4075f240556292", "trace.span_id": "aaaaaaaaaaaaaaaa", "trace.parent_id": "b8d1d0affba3d6c2", "duration_ms": 1000, "service.name": "demo-svc"}},
			{"samplerate": 1, "data": {"name": "test-span", "trace.trace_id": "854b46f148a643a2ed4075f240556292", "trace.span_id": "bbbbbbbbbbbbbbbb", "trace.parent_id": "b8d1d0affba3d6c2", "duration_ms": 2000, "service.name": "demo-svc"}},
			{"samplerate": 1, "data": {"name": "test-span", "trace.trace_id": "854b46f148a643a2ed4075f240556292", "trace.span_id": "cccccccccccccccc", "trace.parent_id": "b8d1d0affba3d6c2", "duration_ms": 1000, "service.name": "demo-svc"}},
			{"samplerate": 1, "data": {"name": "test-span", "trace.trace_id": "854b46f148a643a2ed4075f240556292", "trace.span_id": "b8d1d0affba3d6c2", "duration_ms": 4000, "service.name": "demo-svc"}}
		]`

		var events []LibhoneyEvent
		err := json.Unmarshal([]byte(jsonBatch), &events)
		require.NoError(t, err)
		require.Len(t, events, 4)

		spans := make([]ptrace.Span, len(events))
		for i, event := range events {
			spans[i] = ptrace.NewSpan()
			err := event.ToPTraceSpan(&spans[i], &alreadyUsedFields, testCfg, *zap.NewNop())
			require.NoError(t, err)
		}

		// BUG: Without time fields, all events get time.Now() and have the same start.
		// All four spans will have approximately the same start timestamp.
		// In a correct trace, span 2 should start 1s after span 1, etc.
		startDiff := int64(spans[1].StartTimestamp()) - int64(spans[0].StartTimestamp())
		assert.Less(t, startDiff, int64(100*time.Millisecond),
			"BUG CONFIRMED: without time fields, spans have nearly identical start timestamps (diff: %v)", time.Duration(startDiff))
	})

	t.Run("JSON batch events WITH time fields preserve distinct timestamps", func(t *testing.T) {
		jsonBatch := `[
			{"time": "2026-01-30T09:36:48Z", "samplerate": 1, "data": {"name": "test-span", "trace.trace_id": "854b46f148a643a2ed4075f240556292", "trace.span_id": "aaaaaaaaaaaaaaaa", "trace.parent_id": "b8d1d0affba3d6c2", "duration_ms": 1000, "service.name": "demo-svc"}},
			{"time": "2026-01-30T09:36:49Z", "samplerate": 1, "data": {"name": "test-span", "trace.trace_id": "854b46f148a643a2ed4075f240556292", "trace.span_id": "bbbbbbbbbbbbbbbb", "trace.parent_id": "b8d1d0affba3d6c2", "duration_ms": 2000, "service.name": "demo-svc"}},
			{"time": "2026-01-30T09:36:51Z", "samplerate": 1, "data": {"name": "test-span", "trace.trace_id": "854b46f148a643a2ed4075f240556292", "trace.span_id": "cccccccccccccccc", "trace.parent_id": "b8d1d0affba3d6c2", "duration_ms": 1000, "service.name": "demo-svc"}},
			{"time": "2026-01-30T09:36:48Z", "samplerate": 1, "data": {"name": "test-span", "trace.trace_id": "854b46f148a643a2ed4075f240556292", "trace.span_id": "b8d1d0affba3d6c2", "duration_ms": 4000, "service.name": "demo-svc"}}
		]`

		var events []LibhoneyEvent
		err := json.Unmarshal([]byte(jsonBatch), &events)
		require.NoError(t, err)
		require.Len(t, events, 4)

		spans := make([]ptrace.Span, len(events))
		for i, event := range events {
			spans[i] = ptrace.NewSpan()
			err := event.ToPTraceSpan(&spans[i], &alreadyUsedFields, testCfg, *zap.NewNop())
			require.NoError(t, err)
		}

		expectedT := time.Date(2026, 1, 30, 9, 36, 48, 0, time.UTC)

		// Span 1: start=T, end=T+1s
		assert.Equal(t, pcommon.NewTimestampFromTime(expectedT), spans[0].StartTimestamp())
		assert.Equal(t, pcommon.NewTimestampFromTime(expectedT.Add(1*time.Second)), spans[0].EndTimestamp())

		// Span 2: start=T+1s, end=T+3s
		assert.Equal(t, pcommon.NewTimestampFromTime(expectedT.Add(1*time.Second)), spans[1].StartTimestamp())
		assert.Equal(t, pcommon.NewTimestampFromTime(expectedT.Add(3*time.Second)), spans[1].EndTimestamp())

		// Span 3: start=T+3s, end=T+4s
		assert.Equal(t, pcommon.NewTimestampFromTime(expectedT.Add(3*time.Second)), spans[2].StartTimestamp())
		assert.Equal(t, pcommon.NewTimestampFromTime(expectedT.Add(4*time.Second)), spans[2].EndTimestamp())

		// Root: start=T, end=T+4s
		assert.Equal(t, pcommon.NewTimestampFromTime(expectedT), spans[3].StartTimestamp())
		assert.Equal(t, pcommon.NewTimestampFromTime(expectedT.Add(4*time.Second)), spans[3].EndTimestamp())

		// The key assertion: spans should have DIFFERENT start timestamps
		assert.NotEqual(t, spans[0].StartTimestamp(), spans[1].StartTimestamp(),
			"span 1 and span 2 should have different start timestamps")
		assert.NotEqual(t, spans[1].StartTimestamp(), spans[2].StartTimestamp(),
			"span 2 and span 3 should have different start timestamps")
	})
}

func TestLibhoneyEvent_UnmarshalMsgpack(t *testing.T) {
	tests := []struct {
		name                  string
		msgpackData           map[string]any
		expectNonNilTimestamp bool
		wantErr               bool
	}{
		{
			name: "msgpack with nil timestamp",
			msgpackData: map[string]any{
				"data": map[string]any{
					"key": "value",
				},
				"samplerate": 1,
				// time field is not set (nil)
			},
			expectNonNilTimestamp: true,
		},
		{
			name: "msgpack with time string in data",
			msgpackData: map[string]any{
				"data": map[string]any{
					"key":  "value",
					"time": "2024-01-01T00:00:00Z",
				},
				"samplerate": 2,
			},
			expectNonNilTimestamp: true,
		},
		{
			name: "msgpack with timestamp field",
			msgpackData: map[string]any{
				"time": time.Now(),
				"data": map[string]any{
					"key": "value",
				},
				"samplerate": 3,
			},
			expectNonNilTimestamp: true,
		},
		{
			name: "msgpack with zero timestamp",
			msgpackData: map[string]any{
				"time": time.Time{},
				"data": map[string]any{
					"key": "value",
				},
				"samplerate": 4,
			},
			expectNonNilTimestamp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal the test data to msgpack
			msgpackBytes, err := msgpack.Marshal(tt.msgpackData)
			require.NoError(t, err)

			// Unmarshal using our custom unmarshaller
			var event LibhoneyEvent
			err = event.UnmarshalMsgpack(msgpackBytes)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			// The key assertion: MsgPackTimestamp should never be nil after unmarshalling
			if tt.expectNonNilTimestamp {
				assert.NotNil(t, event.MsgPackTimestamp, "MsgPackTimestamp should never be nil after UnmarshalMsgpack")

				// Additional checks
				if event.MsgPackTimestamp != nil && !event.MsgPackTimestamp.IsZero() {
					// If we have a valid timestamp, Time string should also be set
					assert.NotEmpty(t, event.Time, "Time string should be set when MsgPackTimestamp is valid")
				}
			}

			// Check that data was preserved
			assert.Equal(t, tt.msgpackData["samplerate"], event.Samplerate)
			if data, ok := tt.msgpackData["data"].(map[string]any); ok {
				// Remove "time" from data if it was extracted
				delete(data, "time")
				if len(data) > 0 {
					for k, v := range data {
						assert.Equal(t, v, event.Data[k])
					}
				}
			}
		})
	}
}
