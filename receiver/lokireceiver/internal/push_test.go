// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"net/http"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/grafana/loki/pkg/push"
)

func TestParseRequest_Encodings(t *testing.T) {
	tests := []struct {
		name            string
		contentEncoding string
		compressData    bool
		expectError     bool
	}{
		{
			name:            "No encoding",
			contentEncoding: "",
			compressData:    false,
			expectError:     false,
		},
		{
			name:            "Snappy encoding",
			contentEncoding: "snappy",
			compressData:    false,
			expectError:     false,
		},
		{
			name:            "Gzip encoding",
			contentEncoding: "gzip",
			compressData:    true,
			expectError:     false,
		},
		{
			name:            "Deflate encoding",
			contentEncoding: "deflate",
			compressData:    true,
			expectError:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jsonData := `{"streams":[{"stream":{"foo":"bar"},"values":[["1680000000000000000","log line"]]}]}`

			var body string
			if tc.compressData {
				if tc.contentEncoding == "gzip" {
					body, _ = compressGzip(jsonData)
				} else if tc.contentEncoding == "deflate" {
					body, _ = compressDeflate(jsonData)
				}
			} else {
				body = jsonData
			}

			req := createTestRequest(body, "application/json", tc.contentEncoding)

			pushReq, err := ParseRequest(req)
			if tc.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.expectError && len(pushReq.Streams) != 1 {
				t.Errorf("expected 1 stream, got %d", len(pushReq.Streams))
			}
		})
	}
}

func TestParseRequest_GzipEncodingError(t *testing.T) {
	// Invalid gzip data
	req := createTestRequest("not-gzip-data", "application/json", "gzip")

	_, err := ParseRequest(req)
	if err == nil {
		t.Error("expected error from invalid gzip data")
	}
}

func TestParseRequest_UnsupportedEncoding(t *testing.T) {
	req := createTestRequest("data", "application/json", "unsupported")

	_, err := ParseRequest(req)
	if err == nil {
		t.Error("expected error for unsupported encoding")
	}
}

func TestParseRequest_InvalidContentType(t *testing.T) {
	req := createTestRequest("data", "invalid/content-type", "")

	_, err := ParseRequest(req)
	if err == nil {
		t.Error("expected error for invalid content type")
	}
}

func TestParseRequest_ProtobufContentType(t *testing.T) {
	// Create a simple protobuf message
	protoData := createTestProtobuf()
	req := createTestRequest(protoData, "application/x-protobuf", "")

	pushReq, err := ParseRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pushReq == nil {
		t.Error("expected non-nil push request")
	}
}

// Helper functions
func createTestRequest(body, contentType, contentEncoding string) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte(body)))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}
	req.ContentLength = int64(len(body))
	return req
}

func compressGzip(data string) (string, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)

	if _, err := gw.Write([]byte(data)); err != nil {
		return "", err
	}

	if err := gw.Close(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func compressDeflate(data string) (string, error) {
	var buf bytes.Buffer
	fw, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return "", err
	}

	if _, err := fw.Write([]byte(data)); err != nil {
		return "", err
	}

	if err := fw.Close(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func createTestProtobuf() string {
	// Create a minimal protobuf message for testing
	// This is a simplified version - in practice you'd use actual protobuf encoding
	req := &push.PushRequest{
		Streams: []push.Stream{
			{
				Labels: "{foo=\"bar\"}",
				Entries: []push.Entry{
					{
						Timestamp: time.Unix(1680000000, 0),
						Line:      "test log",
					},
				},
			},
		},
	}

	// Serializar y comprimir con snappy
	data, _ := req.Marshal()
	compressed := snappy.Encode(nil, data)
	return string(compressed)
}
