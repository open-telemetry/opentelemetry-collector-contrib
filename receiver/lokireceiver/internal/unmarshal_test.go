// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"testing"

	"github.com/grafana/loki/pkg/push"
)

func TestStream_UnmarshalJSON_Errors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "Malformed JSON",
			input:       `{"stream":{"foo":"bar"`,
			expectError: true,
		},
		{
			name:        "Stream is not a JSON object",
			input:       `{"stream": "not-a-json-object", "values": [["1680000000000000000","log line"]]}`,
			expectError: true,
		},
		{
			name:        "Values is not an array",
			input:       `{"stream":{"foo":"bar"}, "values": "not-an-array"}`,
			expectError: true,
		},
		{
			name:        "Valid JSON with null values",
			input:       `{"stream":{"foo":"bar"}, "values": null}`,
			expectError: false,
		},
		{
			name:        "Valid JSON",
			input:       `{"stream":{"foo":"bar"}, "values": [["1680000000000000000","log line"]]}`,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var s Stream
			err := s.UnmarshalJSON([]byte(tc.input))
			if tc.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLabelSet_UnmarshalJSON_and_String(t *testing.T) {
	jsonData := []byte(`{"foo":"bar","baz":"qux"}`)
	var ls LabelSet

	err := ls.UnmarshalJSON(jsonData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	str := ls.String()
	if str != "{baz=\"qux\", foo=\"bar\"}" && str != "{foo=\"bar\", baz=\"qux\"}" {
		t.Errorf("unexpected string output: %s", str)
	}
}

func TestLabelSet_UnmarshalJSON_Invalid(t *testing.T) {
	var ls LabelSet

	_ = ls.UnmarshalJSON([]byte(`{"foo":123}`))
	if ls["foo"] != "123" {
		t.Errorf("expected value to be '123', got %q", ls["foo"])
	}
}

func TestUnmarshalHTTPToLogProtoEntries(t *testing.T) {
	jsonData := []byte(`[["1680000000000000000","log line"]]`)
	entries, err := unmarshalHTTPToLogProtoEntries(jsonData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestUnmarshalHTTPToLogProtoEntry(t *testing.T) {
	jsonData := []byte(`["1680000000000000000","log line"]`)
	entry, err := unmarshalHTTPToLogProtoEntry(jsonData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Line != "log line" {
		t.Errorf("unexpected line: %s", entry.Line)
	}
	if entry.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestDecodePushRequest(t *testing.T) {
	jsonData := []byte(`{"streams":[{"stream":{"foo":"bar"},"values":[["1680000000000000000","log line"]]}]}`)
	var req push.PushRequest
	err := decodePushRequest(bytes.NewReader(jsonData), &req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(req.Streams) != 1 {
		t.Errorf("expected 1 stream, got %d", len(req.Streams))
	}
}

func TestUnmarshalHTTPToLogProtoEntry_Invalid(t *testing.T) {
	// Not an array
	_, err := unmarshalHTTPToLogProtoEntry([]byte(`{"foo": "bar"}`))
	if err == nil {
		t.Error("expected error for non-array input")
	}
	// Wrong types
	_, err = unmarshalHTTPToLogProtoEntry([]byte(`[123, 456]`))
	if err == nil {
		t.Error("expected error for wrong types")
	}
}
