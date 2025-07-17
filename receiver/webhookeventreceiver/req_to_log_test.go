// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver/internal/metadata"
)

func TestReqToLog(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)

	tests := []struct {
		desc    string
		sc      *bufio.Scanner
		headers http.Header
		query   url.Values
		config  *Config
		tt      func(t *testing.T, reqLog plog.Logs, reqLen int, settings receiver.Settings)
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
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, _ receiver.Settings) {
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
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, _ receiver.Settings) {
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
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, _ receiver.Settings) {
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
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, _ receiver.Settings) {
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
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, _ receiver.Settings) {
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
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, _ receiver.Settings) {
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
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, _ receiver.Settings) {
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
			tt: func(t *testing.T, _ plog.Logs, reqLen int, _ receiver.Settings) {
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
			tt: func(t *testing.T, _ plog.Logs, reqLen int, _ receiver.Settings) {
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
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, _ receiver.Settings) {
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
			tt: func(t *testing.T, reqLog plog.Logs, reqLen int, _ receiver.Settings) {
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
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			testConfig := defaultConfig
			if test.config != nil {
				testConfig = test.config
			}

			// receiver will fail to create if endpoint is empty
			testConfig.Endpoint = "localhost:8080"
			receiver, err := newLogsReceiver(receivertest.NewNopSettings(metadata.Type), *testConfig, consumertest.NewNop())
			require.NoError(t, err)
			eventReceiver := receiver.(*eventReceiver)
			defer func() {
				err := eventReceiver.Shutdown(t.Context())
				require.NoError(t, err)
			}()

			reqLog, reqLen := eventReceiver.reqToLog(test.sc, test.headers, test.query)
			test.tt(t, reqLog, reqLen, receivertest.NewNopSettings(metadata.Type))
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

// TestSplitJSONObjectsComparison compares the performance and results of regex vs decoder approaches
func TestSplitJSONObjectsComparison(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  int // expected number of JSON objects
	}{
		{
			name:  "single JSON object",
			input: `{"name": "john", "age": 30}`,
			want:  1,
		},
		{
			name: "two JSON objects with newline",
			input: `{"name": "john", "age": 30}
{"name": "jane", "age": 25}`,
			want: 2,
		},
		{
			name:  "two JSON objects without space",
			input: `{"name": "john", "age": 30}{"name": "jane", "age": 25}`,
			want:  2,
		},
		{
			name: "multiple JSON objects with spaces",
			input: `{"name": "john", "age": 30}   {"name": "jane", "age": 25}
			{"name": "bob", "city": "NYC"}`,
			want: 3,
		},
		{
			name: "nested JSON objects",
			input: `{"user": {"name": "john", "address": {"city": "NYC"}}}
{"user": {"name": "jane", "address": {"city": "LA"}}}`,
			want: 2,
		},
		{
			name:  "array of values",
			input: `{"items": [1, 2, 3]}{"items": [4, 5, 6]}`,
			want:  2,
		},
		{
			name:  "mixed valid and invalid JSON",
			input: `{"valid": true}not json{"another": "valid"}`,
			want:  2, // regex will split all 3, decoder will only get valid JSON
		},
		{
			name:  "empty input",
			input: ``,
			want:  1, // both return the original string as fallback
		},
		{
			name:  "single invalid JSON",
			input: `not a json object at all`,
			want:  1, // both return the original string as fallback
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Time regex approach
			start := time.Now()
			regexResult := splitJSONObjects(tc.input)
			regexDuration := time.Since(start)

			// Time decoder approach
			start = time.Now()
			decoderResult := splitJSONObjectsDecoder(tc.input)
			decoderDuration := time.Since(start)

			t.Logf("Input: %q", tc.input)
			t.Logf("Regex approach: %d objects in %v", len(regexResult), regexDuration)
			t.Logf("Decoder approach: %d objects in %v", len(decoderResult), decoderDuration)

			// Log the actual results for debugging
			t.Logf("Regex results:")
			for i, obj := range regexResult {
				t.Logf("  [%d]: %q", i, obj)
			}
			t.Logf("Decoder results:")
			for i, obj := range decoderResult {
				t.Logf("  [%d]: %q", i, obj)
			}

			// Note: The two approaches may give different results for invalid JSON
			// This is expected behavior - decoder is more strict
			if tc.name != "mixed valid and invalid JSON" {
				// For valid JSON cases, we expect similar counts
				if len(regexResult) != len(decoderResult) {
					t.Logf("Warning: Different result counts - regex: %d, decoder: %d",
						len(regexResult), len(decoderResult))
				}
			}
		})
	}
}

// BenchmarkSplitJSONObjects benchmarks both approaches with various input sizes
func BenchmarkSplitJSONObjects(b *testing.B) {
	// Generate test data with different sizes
	generateJSONObjects := func(count int) string {
		var builder strings.Builder
		for i := 0; i < count; i++ {
			if i > 0 {
				builder.WriteString("\n")
			}
			builder.WriteString(fmt.Sprintf(`{"id": %d, "name": "user%d", "timestamp": %d}`, i, i, time.Now().Unix()))
		}
		return builder.String()
	}

	testSizes := []int{1, 10, 100, 1000}

	for _, size := range testSizes {
		input := generateJSONObjects(size)

		b.Run(fmt.Sprintf("Regex_%d_newlines", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = splitJSONObjects(input)
			}
		})

		b.Run(fmt.Sprintf("Decoder_%d_newlines", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = splitJSONObjectsDecoder(input)
			}
		})
	}

	// Test with concatenated objects (no separators)
	for _, size := range testSizes {
		var builder strings.Builder
		for i := 0; i < size; i++ {
			builder.WriteString(fmt.Sprintf(`{"id": %d, "name": "user%d", "timestamp": %d}`, i, i, time.Now().Unix()))
		}
		input := builder.String()

		b.Run(fmt.Sprintf("Regex_%d_single_line", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = splitJSONObjects(input)
			}
		})

		b.Run(fmt.Sprintf("Decoder_%d_single_line", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = splitJSONObjectsDecoder(input)
			}
		})
	}
}
