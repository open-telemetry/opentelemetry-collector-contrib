// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyevent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
			want:        "libhoney.receiver",
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
