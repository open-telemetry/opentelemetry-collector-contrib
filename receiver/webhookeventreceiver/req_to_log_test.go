// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net/http"
	"net/textproto"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver/internal/metadata"
)

func TestReqToLog(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)

	tests := []struct {
		desc        string
		sc          *bufio.Scanner
		headers     http.Header
		query       url.Values
		config      *Config
		expectError bool
		tt          func(t *testing.T, reqLog plog.Logs, reqLen int, err error, settings receiver.Settings)
	}{
		{
			desc: "Valid query valid event",
			sc: func() *bufio.Scanner {
				reader := io.NopCloser(bytes.NewReader([]byte("this is a: log")))
				return bufio.NewScanner(reader)
			}(),
			query: func() url.Values {
				v, err := url.ParseQuery(`qparam1=hello&qparam2=world`)
				if err != nil {
					log.Fatal("failed to parse query")
				}
				return v
			}(),
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				require.Equal(t, 1, reqLen)

				attributes := reqLog.ResourceLogs().At(0).Resource().Attributes()
				require.Equal(t, 2, attributes.Len())

				scopeLogsScope := reqLog.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
				require.Equal(t, 2, scopeLogsScope.Attributes().Len())

				if v, ok := attributes.Get("qparam1"); ok {
					require.Equal(t, "hello", v.AsString())
				} else {
					require.Fail(t, "failed to set attribute from query parameter 1")
				}
				if v, ok := attributes.Get("qparam2"); ok {
					require.Equal(t, "world", v.AsString())
				} else {
					require.Fail(t, "failed to set attribute query parameter 2")
				}
			},
		},
		{
			desc: "new lines present in body",
			sc: func() *bufio.Scanner {
				reader := io.NopCloser(bytes.NewReader([]byte("{\n\"key\":\"value\"\n}")))
				return bufio.NewScanner(reader)
			}(),
			query: func() url.Values {
				v, err := url.ParseQuery(`qparam1=hello&qparam2=world`)
				if err != nil {
					log.Fatal("failed to parse query")
				}
				return v
			}(),
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				require.Equal(t, 1, reqLen)

				attributes := reqLog.ResourceLogs().At(0).Resource().Attributes()
				require.Equal(t, 2, attributes.Len())

				scopeLogsScope := reqLog.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
				require.Equal(t, 2, scopeLogsScope.Attributes().Len())

				if v, ok := attributes.Get("qparam1"); ok {
					require.Equal(t, "hello", v.AsString())
				} else {
					require.Fail(t, "failed to set attribute from query parameter 1")
				}
				if v, ok := attributes.Get("qparam2"); ok {
					require.Equal(t, "world", v.AsString())
				} else {
					require.Fail(t, "failed to set attribute query parameter 2")
				}
			},
		},
		{
			desc: "Query is empty",
			sc: func() *bufio.Scanner {
				reader := io.NopCloser(bytes.NewReader([]byte("this is a: log")))
				return bufio.NewScanner(reader)
			}(),
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				require.Equal(t, 1, reqLen)

				attributes := reqLog.ResourceLogs().At(0).Resource().Attributes()
				require.Equal(t, 0, attributes.Len())

				scopeLogsScope := reqLog.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
				require.Equal(t, 2, scopeLogsScope.Attributes().Len())
			},
		},
		{
			desc: "Headers not added by default",
			headers: http.Header{
				textproto.CanonicalMIMEHeaderKey("X-Foo"): []string{"1"},
				textproto.CanonicalMIMEHeaderKey("X-Bar"): []string{"2"},
			},
			sc: func() *bufio.Scanner {
				reader := io.NopCloser(bytes.NewReader([]byte("this is a: log")))
				return bufio.NewScanner(reader)
			}(),
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				require.Equal(t, 1, reqLen)

				attributes := reqLog.ResourceLogs().At(0).Resource().Attributes()
				require.Equal(t, 0, attributes.Len())

				scopeLogsScope := reqLog.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
				require.Equal(t, 2, scopeLogsScope.Attributes().Len())

				processLogRecords(reqLog, func(lr plog.LogRecord) {
					// expect no additional attributes even though headers are set
					require.Equal(t, 0, lr.Attributes().Len())
				})
			},
		},
		{
			desc:    "HeaderAttributeRegex enabled but no headers included",
			headers: http.Header{},
			config: &Config{
				Path:                 defaultPath,
				HealthPath:           defaultHealthPath,
				ReadTimeout:          defaultReadTimeout,
				WriteTimeout:         defaultWriteTimeout,
				RequiredHeader:       RequiredHeader{Key: "X-Required-Header", Value: "password"},
				HeaderAttributeRegex: ".+",
			},
			sc: func() *bufio.Scanner {
				reader := io.NopCloser(bytes.NewReader([]byte("this is a: log")))
				return bufio.NewScanner(reader)
			}(),
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				require.Equal(t, 1, reqLen)

				attributes := reqLog.ResourceLogs().At(0).Resource().Attributes()
				require.Equal(t, 0, attributes.Len())

				scopeLogsScope := reqLog.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
				require.Equal(t, 2, scopeLogsScope.Attributes().Len())

				processLogRecords(reqLog, func(lr plog.LogRecord) {
					// expect no additional attributes because no headers were set
					require.Equal(t, 0, lr.Attributes().Len())
				})
			},
		},
		{
			desc: "Headers added if HeaderAttributeRegex enabled",
			headers: http.Header{
				textproto.CanonicalMIMEHeaderKey("X-Foo"): []string{"1"},
				textproto.CanonicalMIMEHeaderKey("X-Bar"): []string{"2", "3"},
			},
			config: &Config{
				Path:                 defaultPath,
				HealthPath:           defaultHealthPath,
				ReadTimeout:          defaultReadTimeout,
				WriteTimeout:         defaultWriteTimeout,
				HeaderAttributeRegex: ".+",
			},
			sc: func() *bufio.Scanner {
				reader := io.NopCloser(bytes.NewReader([]byte("this is a: log")))
				return bufio.NewScanner(reader)
			}(),
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				require.Equal(t, 1, reqLen)

				attributes := reqLog.ResourceLogs().At(0).Resource().Attributes()
				require.Equal(t, 0, attributes.Len())

				scopeLogsScope := reqLog.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
				require.Equal(t, 2, scopeLogsScope.Attributes().Len())

				processLogRecords(reqLog, func(lr plog.LogRecord) {
					// expect no additional attributes even though headers are set
					require.Equal(t, 2, lr.Attributes().Len())
					v, exists := lr.Attributes().Get("header.X-Foo")
					require.True(t, exists)
					require.Equal(t, "1", v.Slice().At(0).AsString())
					v, exists = lr.Attributes().Get("header.X-Bar")
					require.True(t, exists)
					require.Equal(t, "2", v.Slice().At(0).AsString())
					require.Equal(t, "3", v.Slice().At(1).AsString())
				})
			},
		},
		{
			desc: "Headers added if HeaderAttributeRegex enabled, partial match",
			headers: http.Header{
				textproto.CanonicalMIMEHeaderKey("X-Foo"):  []string{"1"},
				textproto.CanonicalMIMEHeaderKey("X-Bar"):  []string{"2", "3"},
				textproto.CanonicalMIMEHeaderKey("X-Fizz"): []string{"4"},
				textproto.CanonicalMIMEHeaderKey("X-Buzz"): []string{"5"},
			},
			config: &Config{
				Path:                 defaultPath,
				HealthPath:           defaultHealthPath,
				ReadTimeout:          defaultReadTimeout,
				WriteTimeout:         defaultWriteTimeout,
				HeaderAttributeRegex: "X-Foo|X-Bar",
			},
			sc: func() *bufio.Scanner {
				reader := io.NopCloser(bytes.NewReader([]byte("this is a: log")))
				return bufio.NewScanner(reader)
			}(),
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				require.Equal(t, 1, reqLen)

				attributes := reqLog.ResourceLogs().At(0).Resource().Attributes()
				require.Equal(t, 0, attributes.Len())

				scopeLogsScope := reqLog.ResourceLogs().At(0).ScopeLogs().At(0).Scope()
				require.Equal(t, 2, scopeLogsScope.Attributes().Len())

				processLogRecords(reqLog, func(lr plog.LogRecord) {
					// X-Fizz and X-Buzz are missing because the regex is specific
					require.Equal(t, 2, lr.Attributes().Len())
					v, exists := lr.Attributes().Get("header.X-Foo")
					require.True(t, exists)
					require.Equal(t, "1", v.Slice().At(0).AsString())
					v, exists = lr.Attributes().Get("header.X-Bar")
					require.True(t, exists)
					require.Equal(t, "2", v.Slice().At(0).AsString())
					require.Equal(t, "3", v.Slice().At(1).AsString())
				})
			},
		},
		{
			desc: "multiple JSON objects in one scan due to missing newline split",
			sc: func() *bufio.Scanner {
				// Simulate input where two JSON objects are separated by a newline,
				// but the scanner is not splitting at newlines.
				reader := io.NopCloser(bytes.NewReader([]byte(`{ "name": "francis", "city": "newyork" }
{ "name": "john", "city": "paris" }`)))
				return bufio.NewScanner(reader)
			}(),
			config: &Config{
				Path:                 defaultPath,
				HealthPath:           defaultHealthPath,
				ReadTimeout:          defaultReadTimeout,
				WriteTimeout:         defaultWriteTimeout,
				RequiredHeader:       RequiredHeader{Key: "X-Required-Header", Value: "password"},
				HeaderAttributeRegex: "",
				SplitLogsAtNewLine:   true,
			},
			tt: func(t *testing.T, _ plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				// If the bug is present, reqLen will be 1 (both objects in one log record).
				// The correct behavior is reqLen == 2 (each object in its own log record).
				require.Equal(t, 2, reqLen)
			},
		},
		{
			desc: "multiple JSON objects in split at JSON boundary",
			sc: func() *bufio.Scanner {
				// Simulate input where two JSON objects are separated by a newline,
				// but the scanner is not splitting at newlines.
				reader := io.NopCloser(bytes.NewReader([]byte(`{
				  "name": "francis", 
				  "city": "newyork" 
				}
                { "name": "john", "city": "paris" }{ "name": "tim", "city": "london" }`)))
				return bufio.NewScanner(reader)
			}(),
			config: &Config{
				Path:                    defaultPath,
				HealthPath:              defaultHealthPath,
				ReadTimeout:             defaultReadTimeout,
				WriteTimeout:            defaultWriteTimeout,
				RequiredHeader:          RequiredHeader{Key: "X-Required-Header", Value: "password"},
				HeaderAttributeRegex:    "",
				SplitLogsAtNewLine:      false,
				SplitLogsAtJSONBoundary: true,
			},
			tt: func(t *testing.T, _ plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				// If the bug is present, reqLen will be 1 (both objects in one log record).
				// The correct behavior is reqLen == 2 (each object in its own log record).
				require.Equal(t, 3, reqLen)
			},
		},
		{
			desc: "single multi-line JSON with JSON boundary splitting enabled",
			sc: func() *bufio.Scanner {
				reader := io.NopCloser(bytes.NewReader([]byte(`{
				  "name": "francis",
				  "address": {
				    "city": "newyork",
				    "zip": "10001",
				    "country": "USA"
				  },
				  "tags": ["developer", "opentelemetry"]
				}`)))
				return bufio.NewScanner(reader)
			}(),
			config: &Config{
				Path:                    defaultPath,
				HealthPath:              defaultHealthPath,
				ReadTimeout:             defaultReadTimeout,
				WriteTimeout:            defaultWriteTimeout,
				RequiredHeader:          RequiredHeader{Key: "X-Required-Header", Value: "password"},
				HeaderAttributeRegex:    "",
				SplitLogsAtNewLine:      false,
				SplitLogsAtJSONBoundary: true,
			},
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				// Should be a single log entry since it's one valid JSON object
				require.Equal(t, 1, reqLen)

				// Verify the log content
				processLogRecords(reqLog, func(lr plog.LogRecord) {
					bodyField := lr.Body()
					body := bodyField.Str()
					require.Contains(t, body, "francis")
					require.Contains(t, body, "newyork")
					require.Contains(t, body, "developer")
				})
			},
		},
		{
			desc: "non-JSON data with JSON boundary splitting enabled",
			sc: func() *bufio.Scanner {
				reader := io.NopCloser(bytes.NewReader([]byte(`This is plain text
				that spans multiple lines
				and has no valid JSON structure.
				It should be treated as a single log entry
				despite having newlines and JSON boundary splitting enabled.`)))
				return bufio.NewScanner(reader)
			}(),
			config: &Config{
				Path:                    defaultPath,
				HealthPath:              defaultHealthPath,
				ReadTimeout:             defaultReadTimeout,
				WriteTimeout:            defaultWriteTimeout,
				RequiredHeader:          RequiredHeader{Key: "X-Required-Header", Value: "password"},
				HeaderAttributeRegex:    "",
				SplitLogsAtNewLine:      false,
				SplitLogsAtJSONBoundary: true,
			},
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				// Should be a single log entry since there are no JSON boundaries
				require.Equal(t, 1, reqLen)

				// Verify the log content
				processLogRecords(reqLog, func(lr plog.LogRecord) {
					bodyField := lr.Body()
					body := bodyField.Str()
					require.Contains(t, body, "plain text")
					require.Contains(t, body, "splitting enabled")
				})
			},
		},
		{
			desc: "large payload over 64KB with newline splitting",
			sc: func() *bufio.Scanner {
				// Create a payload larger than default bufio.Scanner max token size (64KB)
				// This tests that the buffer size increase fix works correctly
				largePayload := make([]byte, 100*1024) // 100KB
				for i := range largePayload {
					largePayload[i] = 'A'
				}
				// Add some newlines to test splitting
				largePayload[50*1024] = '\n'
				reader := io.NopCloser(bytes.NewReader(largePayload))
				return bufio.NewScanner(reader)
			}(),
			config: &Config{
				ServerConfig: confighttp.ServerConfig{
					MaxRequestBodySize: 150 * 1024, // Set to 150KB to handle the 100KB test payload
				},
				Path:               defaultPath,
				HealthPath:         defaultHealthPath,
				ReadTimeout:        defaultReadTimeout,
				WriteTimeout:       defaultWriteTimeout,
				SplitLogsAtNewLine: true,
			},
			tt: func(t *testing.T, _ plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				// Should be 2 log entries since there's a newline in the middle
				require.Equal(t, 2, reqLen)
			},
		},
		{
			desc: "large payload over 64KB with JSON boundary splitting",
			sc: func() *bufio.Scanner {
				// Create multiple JSON objects that together exceed 64KB
				// This reproduces the issue from PR #41350 where large payloads fail
				var buf bytes.Buffer
				for i := range 800 {
					// Each JSON object is ~100 bytes, total ~80KB (exceeds 64KB default limit)
					buf.WriteString(`{"event":"webhook","index":`)
					buf.WriteString(string(rune('0' + (i % 10))))
					buf.WriteString(`,"data":"`)
					// Add padding
					for range 60 {
						buf.WriteByte('X')
					}
					buf.WriteString(`"}`)
				}
				reader := io.NopCloser(&buf)
				return bufio.NewScanner(reader)
			}(),
			config: &Config{
				ServerConfig: confighttp.ServerConfig{
					MaxRequestBodySize: 100 * 1024, // 100KB to handle the ~80KB test payload
				},
				Path:                    defaultPath,
				HealthPath:              defaultHealthPath,
				ReadTimeout:             defaultReadTimeout,
				WriteTimeout:            defaultWriteTimeout,
				SplitLogsAtJSONBoundary: true,
			},
			tt: func(t *testing.T, _ plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				// Should be 800 log entries (one per JSON object)
				require.Equal(t, 800, reqLen)
			},
		},
		{
			desc: "request body exceeds max size returns error",
			sc: func() *bufio.Scanner {
				// Create a payload larger than our test config max size
				largePayload := make([]byte, 150*1024) // 150KB
				for i := range largePayload {
					largePayload[i] = 'X'
				}
				reader := io.NopCloser(bytes.NewReader(largePayload))
				return bufio.NewScanner(reader)
			}(),
			config: &Config{
				ServerConfig: confighttp.ServerConfig{
					MaxRequestBodySize: 100 * 1024, // Set to 100KB, smaller than payload
				},
				Path:               defaultPath,
				HealthPath:         defaultHealthPath,
				ReadTimeout:        defaultReadTimeout,
				WriteTimeout:       defaultWriteTimeout,
				SplitLogsAtNewLine: true,
			},
			expectError: true,
			tt: func(t *testing.T, _ plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.Error(t, err)
				require.ErrorIs(t, err, errRequestBodyTooLarge)
				require.Contains(t, err.Error(), "limit is 102400 bytes")
				// Should process 0 logs when error occurs
				require.Equal(t, 0, reqLen)
			},
		},
		{
			desc: "request body with tiny MaxRequestBodySize uses default",
			sc: func() *bufio.Scanner {
				largePayload := make([]byte, 60*1024)
				for i := range largePayload {
					largePayload[i] = 'X'
				}
				reader := io.NopCloser(bytes.NewReader(largePayload))
				return bufio.NewScanner(reader)
			}(),
			config: &Config{
				ServerConfig: confighttp.ServerConfig{
					MaxRequestBodySize: 64, // Set smaller than allowed
				},
				Path:               defaultPath,
				HealthPath:         defaultHealthPath,
				ReadTimeout:        defaultReadTimeout,
				WriteTimeout:       defaultWriteTimeout,
				SplitLogsAtNewLine: true,
			},
			tt: func(t *testing.T, _ plog.Logs, reqLen int, err error, _ receiver.Settings) {
				require.NoError(t, err)
				require.Equal(t, 1, reqLen)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			testConfig := defaultConfig
			if test.config != nil {
				testConfig = test.config
			}
			// Intentionally ignore config validation errors as they're not relevant for this test
			_ = testConfig.Validate()

			// receiver will fail to create if endpoint is empty
			testConfig.NetAddr.Endpoint = "localhost:8080"
			receiver, err := newLogsReceiver(receivertest.NewNopSettings(metadata.Type), *testConfig, consumertest.NewNop())
			require.NoError(t, err)
			eventReceiver := receiver.(*eventReceiver)
			defer func() {
				shutdownErr := eventReceiver.Shutdown(t.Context())
				require.NoError(t, shutdownErr)
			}()

			reqLog, reqLen, err := eventReceiver.reqToLog(test.sc, test.headers, test.query)
			test.tt(t, reqLog, reqLen, err, receivertest.NewNopSettings(metadata.Type))
		})
	}
}

func TestHeaderAttributeKey(t *testing.T) {
	// Test mix of header values to ensure consistent output
	require.Equal(t, "header.foo", headerAttributeKey("foo"))
	require.Equal(t, "header.1", headerAttributeKey("1"))
	require.Equal(t, "header.Content-type", headerAttributeKey("Content-type"))
	require.Equal(t, "header.UnExPectEd-CaMeL-CaSe-HeAdEr", headerAttributeKey("UnExPectEd-CaMeL-CaSe-HeAdEr"))
}

// helper to run a func against each log record
func processLogRecords(logs plog.Logs, fn func(lr plog.LogRecord)) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				fn(sl.LogRecords().At(k))
			}
		}
	}
}
