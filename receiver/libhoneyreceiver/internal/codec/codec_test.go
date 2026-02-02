// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/response"
)

func TestDecodeEvents_JSON(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		headers  http.Header
		wantLen  int
		wantErr  bool
		validate func(t *testing.T, events []any)
	}{
		{
			name:    "single flat event",
			body:    `{"field1": "value1", "field2": 42}`,
			headers: http.Header{},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "single structured event",
			body:    `{"data": {"field1": "value1"}, "time": "2024-01-01T00:00:00Z", "samplerate": 1}`,
			headers: http.Header{},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "array of events",
			body:    `[{"data": {"field1": "value1"}}, {"data": {"field2": "value2"}}]`,
			headers: http.Header{},
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "single event with headers",
			body: `{"field1": "value1"}`,
			headers: http.Header{
				"X-Honeycomb-Event-Time": []string{"2024-01-01T00:00:00Z"},
				"X-Honeycomb-Samplerate": []string{"10"},
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "invalid json",
			body:    `{invalid json}`,
			headers: http.Header{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := DecodeEvents(JSONContentType, []byte(tt.body), tt.headers)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, events, tt.wantLen)

			// Verify all events have MsgPackTimestamp set
			for i, event := range events {
				assert.NotNil(t, event.MsgPackTimestamp, "event %d should have timestamp", i)
			}
		})
	}
}

func TestDecodeEvents_Msgpack(t *testing.T) {
	tests := []struct {
		name    string
		body    func() []byte
		headers http.Header
		wantLen int
		wantErr bool
	}{
		{
			name: "single flat event",
			body: func() []byte {
				data := map[string]any{"field1": "value1", "field2": 42}
				buf, _ := msgpack.Marshal(data)
				return buf
			},
			headers: http.Header{},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "single structured event",
			body: func() []byte {
				data := map[string]any{
					"data":       map[string]any{"field1": "value1"},
					"time":       "2024-01-01T00:00:00Z",
					"samplerate": 1,
				}
				buf, _ := msgpack.Marshal(data)
				return buf
			},
			headers: http.Header{},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "array of events",
			body: func() []byte {
				data := []map[string]any{
					{"data": map[string]any{"field1": "value1"}},
					{"data": map[string]any{"field2": "value2"}},
				}
				buf, _ := msgpack.Marshal(data)
				return buf
			},
			headers: http.Header{},
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "single event with headers",
			body: func() []byte {
				data := map[string]any{"field1": "value1"}
				buf, _ := msgpack.Marshal(data)
				return buf
			},
			headers: http.Header{
				"X-Honeycomb-Event-Time": []string{"2024-01-01T00:00:00Z"},
				"X-Honeycomb-Samplerate": []string{"10"},
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "invalid msgpack",
			body: func() []byte {
				return []byte{0xff, 0xff, 0xff}
			},
			headers: http.Header{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := DecodeEvents("application/msgpack", tt.body(), tt.headers)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, events, tt.wantLen)

			// Verify all events have MsgPackTimestamp set
			for i, event := range events {
				assert.NotNil(t, event.MsgPackTimestamp, "event %d should have timestamp", i)
			}
		})
	}
}

func TestDecodeEvents_UnsupportedContentType(t *testing.T) {
	_, err := DecodeEvents("application/xml", []byte("<xml/>"), http.Header{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported content type")
}

func TestIsSingleMsgpackObject(t *testing.T) {
	tests := []struct {
		name     string
		body     func() []byte
		expected bool
	}{
		{
			name: "fixmap",
			body: func() []byte {
				data := map[string]any{"key": "value"}
				buf, _ := msgpack.Marshal(data)
				return buf
			},
			expected: true,
		},
		{
			name: "fixarray",
			body: func() []byte {
				data := []any{"item1", "item2"}
				buf, _ := msgpack.Marshal(data)
				return buf
			},
			expected: false,
		},
		{
			name: "empty",
			body: func() []byte {
				return []byte{}
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSingleMsgpackObject(tt.body())
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWrapFlatEventIfNeeded(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name: "already has data field",
			input: map[string]any{
				"data":       map[string]any{"field1": "value1"},
				"time":       "2024-01-01T00:00:00Z",
				"samplerate": 1,
			},
			expected: map[string]any{
				"data":       map[string]any{"field1": "value1"},
				"time":       "2024-01-01T00:00:00Z",
				"samplerate": 1,
			},
		},
		{
			name: "flat event without data field",
			input: map[string]any{
				"field1":     "value1",
				"field2":     42,
				"time":       "2024-01-01T00:00:00Z",
				"samplerate": 1,
			},
			expected: map[string]any{
				"data": map[string]any{
					"field1": "value1",
					"field2": 42,
				},
				"time":       "2024-01-01T00:00:00Z",
				"samplerate": 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapFlatEventIfNeeded(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyHeadersToEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    map[string]any
		headers  http.Header
		expected map[string]any
	}{
		{
			name:  "apply both headers",
			event: map[string]any{"field1": "value1"},
			headers: http.Header{
				"X-Honeycomb-Event-Time": []string{"2024-01-01T00:00:00Z"},
				"X-Honeycomb-Samplerate": []string{"10"},
			},
			expected: map[string]any{
				"field1":     "value1",
				"time":       "2024-01-01T00:00:00Z",
				"samplerate": int64(10),
			},
		},
		{
			name: "don't override existing fields",
			event: map[string]any{
				"field1":     "value1",
				"time":       "2024-02-01T00:00:00Z",
				"samplerate": 5,
			},
			headers: http.Header{
				"X-Honeycomb-Event-Time": []string{"2024-01-01T00:00:00Z"},
				"X-Honeycomb-Samplerate": []string{"10"},
			},
			expected: map[string]any{
				"field1":     "value1",
				"time":       "2024-02-01T00:00:00Z",
				"samplerate": 5,
			},
		},
		{
			name:     "no headers",
			event:    map[string]any{"field1": "value1"},
			headers:  http.Header{},
			expected: map[string]any{"field1": "value1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyHeadersToEvent(tt.event, tt.headers)
			assert.Equal(t, tt.expected, tt.event)
		})
	}
}

func TestBuildLibhoneyEventFromMap(t *testing.T) {
	tests := []struct {
		name      string
		rawEvent  map[string]any
		checkTime bool
	}{
		{
			name: "complete event",
			rawEvent: map[string]any{
				"data":       map[string]any{"field1": "value1"},
				"time":       "2024-01-01T00:00:00Z",
				"samplerate": int64(10),
			},
			checkTime: true,
		},
		{
			name: "event without time",
			rawEvent: map[string]any{
				"data":       map[string]any{"field1": "value1"},
				"samplerate": int64(5),
			},
			checkTime: true,
		},
		{
			name: "minimal event",
			rawEvent: map[string]any{
				"data": map[string]any{"field1": "value1"},
			},
			checkTime: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := buildLibhoneyEventFromMap(tt.rawEvent)

			// Check that data is set
			assert.NotNil(t, event.Data)

			// Check that timestamp is always set
			if tt.checkTime {
				assert.NotNil(t, event.MsgPackTimestamp)
			}

			// Check samplerate defaults to 1 if not provided
			if _, ok := tt.rawEvent["samplerate"]; !ok {
				assert.Equal(t, 1, event.Samplerate)
			}
		})
	}
}

func TestDecodeMsgpack_PreservesTypes(t *testing.T) {
	// Test that msgpack decoding preserves different types correctly
	testData := map[string]any{
		"data": map[string]any{
			"string_field": "test",
			"int_field":    int64(42),
			"float_field":  3.14,
			"bool_field":   true,
		},
		"samplerate": int64(10),
		"time":       "2024-01-01T00:00:00Z",
	}

	var buf bytes.Buffer
	msgpackEncoder := msgpack.NewEncoder(&buf)
	err := msgpackEncoder.Encode(testData)
	require.NoError(t, err)

	events, err := DecodeEvents("application/msgpack", buf.Bytes(), http.Header{})
	require.NoError(t, err)
	require.Len(t, events, 1)

	event := events[0]
	assert.Equal(t, 10, event.Samplerate)
	assert.NotNil(t, event.MsgPackTimestamp)
	assert.NotNil(t, event.Data)

	// Verify data fields
	data := event.Data
	assert.Equal(t, "test", data["string_field"])
	assert.Equal(t, int64(42), data["int_field"])
	assert.Equal(t, 3.14, data["float_field"])
	assert.Equal(t, true, data["bool_field"])
}

func TestMarshalResponse_JSON(t *testing.T) {
	tests := []struct {
		name      string
		responses []response.ResponseInBatch
		wantErr   bool
	}{
		{
			name: "success responses",
			responses: []response.ResponseInBatch{
				{Status: 202},
				{Status: 202},
			},
			wantErr: false,
		},
		{
			name: "mixed success and error",
			responses: []response.ResponseInBatch{
				{Status: 202},
				{Status: 400, ErrorStr: "bad request"},
			},
			wantErr: false,
		},
		{
			name: "single response",
			responses: []response.ResponseInBatch{
				{Status: 202},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := JsEncoder.MarshalResponse(tt.responses)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify it's valid JSON
			var decoded []response.ResponseInBatch
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, tt.responses, decoded)

			// Verify content type
			assert.Equal(t, JSONContentType, JsEncoder.ContentType())
		})
	}
}

func TestMarshalResponse_Msgpack(t *testing.T) {
	tests := []struct {
		name      string
		responses []response.ResponseInBatch
		wantErr   bool
	}{
		{
			name: "success responses",
			responses: []response.ResponseInBatch{
				{Status: 202},
				{Status: 202},
			},
			wantErr: false,
		},
		{
			name: "mixed success and error",
			responses: []response.ResponseInBatch{
				{Status: 202},
				{Status: 400, ErrorStr: "bad request"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := MpEncoder.MarshalResponse(tt.responses)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify it's valid msgpack
			var decoded []response.ResponseInBatch
			decoder := msgpack.NewDecoder(bytes.NewReader(data))
			decoder.UseLooseInterfaceDecoding(true)
			err = decoder.Decode(&decoded)
			require.NoError(t, err)
			assert.Equal(t, tt.responses, decoded)

			// Verify content type
			assert.Equal(t, MsgpackContentType, MpEncoder.ContentType())
		})
	}
}

func TestGetEncoder(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantEncoder Encoder
		wantErr     bool
	}{
		{
			name:        "JSON content type",
			contentType: "application/json",
			wantEncoder: JsEncoder,
			wantErr:     false,
		},
		{
			name:        "JSON content type (constant)",
			contentType: JSONContentType,
			wantEncoder: JsEncoder,
			wantErr:     false,
		},
		{
			name:        "Msgpack content type",
			contentType: "application/msgpack",
			wantEncoder: MpEncoder,
			wantErr:     false,
		},
		{
			name:        "Msgpack content type (x-msgpack)",
			contentType: "application/x-msgpack",
			wantEncoder: MpEncoder,
			wantErr:     false,
		},
		{
			name:        "unsupported content type",
			contentType: "application/xml",
			wantErr:     true,
		},
		{
			name:        "empty content type",
			contentType: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder, err := GetEncoder(tt.contentType)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unsupported content type")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantEncoder, encoder)
		})
	}
}

func TestBuildLibhoneyEventFromMap_SamplerateTypes(t *testing.T) {
	tests := []struct {
		name               string
		rawEvent           map[string]any
		expectedSampleRate int
	}{
		{
			name: "samplerate as int",
			rawEvent: map[string]any{
				"data":       map[string]any{"field1": "value1"},
				"samplerate": int(10),
			},
			expectedSampleRate: 10,
		},
		{
			name: "samplerate as int64",
			rawEvent: map[string]any{
				"data":       map[string]any{"field1": "value1"},
				"samplerate": int64(10),
			},
			expectedSampleRate: 10,
		},
		{
			name: "samplerate as uint64",
			rawEvent: map[string]any{
				"data":       map[string]any{"field1": "value1"},
				"samplerate": uint64(10),
			},
			expectedSampleRate: 10,
		},
		{
			name: "samplerate as float64",
			rawEvent: map[string]any{
				"data":       map[string]any{"field1": "value1"},
				"samplerate": float64(10.0),
			},
			expectedSampleRate: 10,
		},
		{
			name: "no samplerate defaults to 1",
			rawEvent: map[string]any{
				"data": map[string]any{"field1": "value1"},
			},
			expectedSampleRate: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := buildLibhoneyEventFromMap(tt.rawEvent)
			assert.Equal(t, tt.expectedSampleRate, event.Samplerate)
		})
	}
}

func TestEncoder_RoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		encoder     Encoder
		contentType string
	}{
		{
			name:        "JSON round trip",
			encoder:     JsEncoder,
			contentType: JSONContentType,
		},
		{
			name:        "Msgpack round trip",
			encoder:     MpEncoder,
			contentType: MsgpackContentType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create response data
			responses := []response.ResponseInBatch{
				{Status: 202},
				{Status: 400, ErrorStr: "test error"},
			}

			// Marshal
			data, err := tt.encoder.MarshalResponse(responses)
			require.NoError(t, err)

			// Unmarshal based on type
			var decoded []response.ResponseInBatch
			if tt.contentType == JSONContentType {
				err = json.Unmarshal(data, &decoded)
			} else {
				decoder := msgpack.NewDecoder(bytes.NewReader(data))
				decoder.UseLooseInterfaceDecoding(true)
				err = decoder.Decode(&decoded)
			}
			require.NoError(t, err)

			// Verify round trip preserved data
			assert.Equal(t, responses, decoded)
		})
	}
}

func TestBuildLibhoneyEventFromMap_NativeTimeValue(t *testing.T) {
	// When the msgpack library decodes a timestamp extension type (ext -1),
	// it produces a time.Time VALUE (not *time.Time pointer) in map[string]any.
	// This test validates that buildLibhoneyEventFromMap handles this correctly.
	// This is the primary path for Refinery → libhoneyreceiver timestamp preservation.

	expectedTime := time.Date(2026, 1, 30, 9, 36, 48, 0, time.UTC)

	t.Run("time.Time value at top level", func(t *testing.T) {
		rawEvent := map[string]any{
			"data":       map[string]any{"field1": "value1"},
			"time":       expectedTime, // time.Time value, as msgpack decodes it
			"samplerate": int64(1),
		}

		event := buildLibhoneyEventFromMap(rawEvent)
		require.NotNil(t, event.MsgPackTimestamp, "MsgPackTimestamp should be set from time.Time value")
		assert.Equal(t, expectedTime, event.MsgPackTimestamp.UTC(),
			"timestamp should match the original time.Time value")
	})

	t.Run("distinct time.Time values in batch produce distinct timestamps", func(t *testing.T) {
		t1 := time.Date(2026, 1, 30, 9, 36, 48, 0, time.UTC)
		t2 := time.Date(2026, 1, 30, 9, 36, 49, 0, time.UTC)
		t3 := time.Date(2026, 1, 30, 9, 36, 51, 0, time.UTC)

		rawEvents := []map[string]any{
			{"data": map[string]any{"name": "span1"}, "time": t1, "samplerate": int64(1)},
			{"data": map[string]any{"name": "span2"}, "time": t2, "samplerate": int64(1)},
			{"data": map[string]any{"name": "span3"}, "time": t3, "samplerate": int64(1)},
		}

		events := make([]time.Time, len(rawEvents))
		for i, raw := range rawEvents {
			event := buildLibhoneyEventFromMap(raw)
			require.NotNil(t, event.MsgPackTimestamp, "event %d should have timestamp", i)
			events[i] = event.MsgPackTimestamp.UTC()
		}

		assert.NotEqual(t, events[0], events[1], "span 1 and 2 should have different timestamps")
		assert.NotEqual(t, events[1], events[2], "span 2 and 3 should have different timestamps")
		assert.Equal(t, time.Second, events[1].Sub(events[0]), "span 2 - span 1 should be 1s")
		assert.Equal(t, 3*time.Second, events[2].Sub(events[0]), "span 3 - span 1 should be 3s")
	})

	t.Run("msgpack round-trip with native time.Time preserves timestamp", func(t *testing.T) {
		// Simulate what Refinery actually sends: msgpack-encoded events with native time.Time
		original := map[string]any{
			"data": map[string]any{
				"name":           "test-span",
				"trace.trace_id": "854b46f148a643a2ed4075f240556292",
				"trace.span_id":  "aaaaaaaaaaaaaaaa",
				"duration_ms":    1000.0,
				"service.name":   "demo-svc",
			},
			"time":       expectedTime,
			"samplerate": int64(1),
		}

		// Encode to msgpack
		var buf bytes.Buffer
		enc := msgpack.NewEncoder(&buf)
		err := enc.Encode(original)
		require.NoError(t, err)

		// Decode as single event (this is what the receiver does)
		events, err := DecodeEvents("application/msgpack", buf.Bytes(), http.Header{})
		require.NoError(t, err)
		require.Len(t, events, 1)

		event := events[0]
		require.NotNil(t, event.MsgPackTimestamp)
		assert.Equal(t, expectedTime, event.MsgPackTimestamp.UTC(),
			"msgpack round-trip should preserve the original timestamp")
	})

	t.Run("msgpack batch round-trip preserves distinct timestamps", func(t *testing.T) {
		t1 := time.Date(2026, 1, 30, 9, 36, 48, 0, time.UTC)
		t2 := time.Date(2026, 1, 30, 9, 36, 49, 0, time.UTC)
		t3 := time.Date(2026, 1, 30, 9, 36, 51, 0, time.UTC)

		batch := []map[string]any{
			{
				"data":       map[string]any{"name": "span1", "trace.trace_id": "aaa", "trace.span_id": "111", "duration_ms": 1000.0},
				"time":       t1,
				"samplerate": int64(1),
			},
			{
				"data":       map[string]any{"name": "span2", "trace.trace_id": "aaa", "trace.span_id": "222", "duration_ms": 2000.0},
				"time":       t2,
				"samplerate": int64(1),
			},
			{
				"data":       map[string]any{"name": "span3", "trace.trace_id": "aaa", "trace.span_id": "333", "duration_ms": 1000.0},
				"time":       t3,
				"samplerate": int64(1),
			},
		}

		// Encode as msgpack array
		var buf bytes.Buffer
		enc := msgpack.NewEncoder(&buf)
		err := enc.Encode(batch)
		require.NoError(t, err)

		// Decode
		events, err := DecodeEvents("application/msgpack", buf.Bytes(), http.Header{})
		require.NoError(t, err)
		require.Len(t, events, 3)

		assert.Equal(t, t1, events[0].MsgPackTimestamp.UTC(), "span 1 time should be T")
		assert.Equal(t, t2, events[1].MsgPackTimestamp.UTC(), "span 2 time should be T+1s")
		assert.Equal(t, t3, events[2].MsgPackTimestamp.UTC(), "span 3 time should be T+3s")

		assert.NotEqual(t, events[0].MsgPackTimestamp, events[1].MsgPackTimestamp,
			"span 1 and 2 should have different timestamps")
	})
}
