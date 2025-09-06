// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestParseRequest(t *testing.T) {
	testCases := []struct {
		name            string
		contentEncoding string
		contentType     string
		input           string
		expectedError   string
	}{
		{
			name:            "Valid JSON input",
			contentEncoding: "",
			contentType:     "application/json",
			input: `[{
				"message": "Test log message",
				"status": "info",
				"timestamp": 1619712000,
				"hostname": "test-host",
				"service": "test-service",
				"ddsource": "test-source",
				"ddtags": "key1:value1,key2:value2"
			}]`,
		},
		{
			name:            "Valid JSON input with nested attributes",
			contentEncoding: "",
			contentType:     "application/json",
			input: `[{
				"message": {"test": "message", "nested": {"test": "nested"}},
				"status": "info",
				"timestamp": 1619712000,
				"hostname": "test-host",
				"service": "test-service",
				"ddsource": "test-source",
				"ddtags": "key1:value1,key2:value2"
			}]`,
		},
		{
			name:            "Valid JSON input with an array as a message",
			contentEncoding: "",
			contentType:     "application/json",
			input: `[{
				"message": ["test", "message"],
				"status": "info",
				"timestamp": 1619712000,
				"hostname": "test-host",
				"service": "test-service",
				"ddsource": "test-source",
				"ddtags": "key1:value1,key2:value2"
			}]`,
		},
		{
			name:            "Gzip encoded input",
			contentEncoding: "gzip",
			contentType:     "application/json",
			input: `[{
				"message": "Gzipped log message",
				"status": "error",
				"timestamp": 1619712000,
				"hostname": "gzip-host",
				"service": "gzip-service",
				"ddsource": "gzip-source",
				"ddtags": "compressed:true"
			}]`,
		},
		{
			name:            "Deflate encoded input",
			contentEncoding: "deflate",
			contentType:     "application/json",
			input: `[{
				"message": "Deflated log message",
				"status": "warn",
				"timestamp": 1619712000,
				"hostname": "deflate-host",
				"service": "deflate-service",
				"ddsource": "deflate-source",
				"ddtags": "compressed:true"
			}]`,
		},
		{
			name:            "Unsupported encoding",
			contentEncoding: "unsupported",
			contentType:     "application/json",
			input:           "",
			expectedError:   "Content-Encoding \"unsupported\" not supported",
		},
		{
			name:            "Malformed JSON input",
			contentEncoding: "gzip",
			contentType:     "application/json",
			input:           `{"incomplete": "json`,
			expectedError:   `failed to parse as array: []internal.DatadogRecord: decode slice: expect [ or n, but found {, error found in #1 byte of ...|{"incomplet|..., bigger context ...|{"incomplete": "json|...; as single record: internal.DatadogRecord.readStringSlowPath: unexpected end of input, error found in #10 byte of ...|te": "json|..., bigger context ...|{"incomplete": "json|...`,
		},
		{
			name:            "Empty input",
			contentEncoding: "gzip",
			contentType:     "application/json",
			input:           "",
			expectedError:   "failed to parse as array: []internal.DatadogRecord: decode slice: expect [ or n, but found \u0000, error found in #0 byte of ...||..., bigger context ...||...; as single record: readObjectStart: expect { or n, but found \u0000, error found in #0 byte of ...||..., bigger context ...||...",
		},
		{
			name:            "Single empty object",
			contentEncoding: "gzip",
			contentType:     "application/json",
			input:           `{}`,
		},
		{
			name:            "Valid JSON input as a single object",
			contentEncoding: "",
			contentType:     "application/json",
			input: `{
				"message": "Test log message",
				"status": "info",
				"timestamp": 1619712000,
				"hostname": "test-host",
				"service": "test-service",
				"ddsource": "test-source",
				"ddtags": "key1:value1,key2:value2"
			}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for i := 0; i <= 1; i++ {
				enableDdtagsAttribute := i == 1
				logger, _ := zap.NewDevelopment()
				logger.Info(
					"test",
					zap.String("contentEncoding", tc.contentEncoding),
					zap.String("contentType", tc.contentType),
					zap.String("message", tc.input),
				)
				req, err := createTestRequest([]byte(tc.input), tc.contentEncoding, tc.contentType)
				require.NoError(t, err)
				logs, err := ParseRequest(req, logger, enableDdtagsAttribute)

				if tc.expectedError != "" {
					assert.EqualError(t, err, tc.expectedError)
					assert.Nil(t, logs)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, logs)
					if logs != nil {
						validateParsedLogs(t, logs, tc.input, enableDdtagsAttribute)
					}
				}
			}
		})
	}
}

func createTestRequest(input []byte, encoding, contentType string) (*http.Request, error) {
	var body io.Reader = bytes.NewReader(input)

	switch encoding {
	case "gzip":
		var buf bytes.Buffer
		gzipWriter := gzip.NewWriter(&buf)
		_, err := gzipWriter.Write(input)
		if err != nil {
			return nil, err
		}
		gzipWriter.Close()
		body = &buf
	case "deflate":
		var buf bytes.Buffer
		flateWriter, err := flate.NewWriter(&buf, flate.DefaultCompression)
		if err != nil {
			return nil, err
		}
		_, err = flateWriter.Write(input)
		if err != nil {
			return nil, err
		}
		flateWriter.Close()
		body = &buf
	}

	req, err := http.NewRequest("POST", "/api/v2/logs", body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)
	if encoding != "" {
		req.Header.Set("Content-Encoding", encoding)
	}

	return req, nil
}

func validateParsedLogs(t *testing.T, logs *plog.Logs, input string, enableDdtagsAttribute bool) {
	var records []DatadogRecord
	if strings.HasPrefix(input, "{") {
		input = "[" + input + "]"
	}
	err := json.Unmarshal([]byte(input), &records)
	require.NoError(t, err)

	assert.Equal(t, len(records), int(logs.LogRecordCount()))

	if len(records) == 0 {
		return
	}

	resource := logs.ResourceLogs().At(0).Resource()
	if enableDdtagsAttribute {
		ddtagsVal, ok := resource.Attributes().Get("ddtags")
		assert.True(t, ok, "ddtags attribute should exist")
		validateDdtags(t, ddtagsVal.Map(), records[0].Ddtags)
	} else {
		validateDdtags(t, resource.Attributes(), records[0].Ddtags)
	}

	logSlice := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	for i, record := range records {
		lr := logSlice.At(i)

		switch {
		case record.Message.Str != "":
			assert.Equal(t, record.Message.Str, lr.Body().Str())
		case record.Message.Map != nil:
			assert.Equal(t, record.Message.Map, lr.Body().Map().AsRaw())
		case record.Message.Arr != nil:
			assert.Equal(t, record.Message.Arr, lr.Body().Slice().AsRaw())
		}

		expectedSeverityNumber, expectedSeverityText := toSeverity(
			record.Status,
			record.Message.Str,
		)
		assert.Equal(t, expectedSeverityNumber, lr.SeverityNumber())
		assert.Equal(t, expectedSeverityText, lr.SeverityText())

		attrs := lr.Attributes()
		hostnameVal, _ := attrs.Get("hostname")
		assert.Equal(t, record.Hostname, hostnameVal.Str())

		serviceVal, _ := attrs.Get("service")
		assert.Equal(t, record.Service, serviceVal.Str())

		statusVal, exists := attrs.Get("status")
		assert.True(t, exists, "status attribute should exist")
		if record.Status == "error" {
			assert.Equal(t, expectedSeverityText, statusVal.Str())
		} else {
			assert.Equal(t, record.Status, statusVal.Str())
		}

		datadogLogSourceVal, _ := attrs.Get("datadog.log.source")
		assert.Equal(t, record.Ddsource, datadogLogSourceVal.Str())
	}
}

func validateDdtags(t *testing.T, attrs pcommon.Map, ddtags string) {
	for _, tag := range strings.Split(ddtags, ",") {
		kv := strings.SplitN(tag, ":", 2)
		if len(kv) == 1 {
			val, exists := attrs.Get(kv[0])
			assert.True(t, exists, "Attribute %s should exist", kv[0])
			assert.Equal(t, "", val.Str())
		} else {
			val, exists := attrs.Get(kv[0])
			assert.True(t, exists, "Attribute %s should exist", kv[0])
			assert.Equal(t, kv[1], val.Str())
		}
	}
}
