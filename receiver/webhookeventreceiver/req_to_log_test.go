// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/textproto"
	"net/url"
	"testing"

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
				err := eventReceiver.Shutdown(context.Background())
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
