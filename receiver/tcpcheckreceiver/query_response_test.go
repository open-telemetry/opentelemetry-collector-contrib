// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

// startServer starts a TCP server on a random local port. Every accepted
// connection is handed to handler and closed when handler returns.
func startServer(t *testing.T, handler func(conn net.Conn)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				handler(conn)
			}()
		}
	}()
	return listener.Addr().String()
}

// checkResult is what a single scrape recorded for a single target.
type checkResult struct {
	status    int64
	errorCode string
	err       error
}

func runCheck(ctx context.Context, t *testing.T, target *TargetConfig) checkResult {
	t.Helper()
	cfg := &Config{Targets: []*TargetConfig{target}}
	require.NoError(t, cfg.Validate())

	s, err := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)

	metrics, err := s.scrape(ctx)
	result := checkResult{err: err, status: -1}
	for _, m := range allMetrics(metrics) {
		switch m.Name() {
		case "tcpcheck.status":
			require.Equal(t, 1, m.Gauge().DataPoints().Len())
			result.status = m.Gauge().DataPoints().At(0).IntValue()
		case "tcpcheck.error":
			require.Equal(t, 1, m.Sum().DataPoints().Len())
			code, ok := m.Sum().DataPoints().At(0).Attributes().Get("error.code")
			require.True(t, ok)
			result.errorCode = code.Str()
		}
	}
	return result
}

func allMetrics(metrics pmetric.Metrics) []pmetric.Metric {
	var out []pmetric.Metric
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		sms := metrics.ResourceMetrics().At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				out = append(out, ms.At(k))
			}
		}
	}
	return out
}

func newTarget(endpoint string, timeout time.Duration, steps ...QueryResponseStep) *TargetConfig {
	return &TargetConfig{
		TCPAddrConfig: confignet.TCPAddrConfig{
			Endpoint:     endpoint,
			DialerConfig: confignet.DialerConfig{Timeout: timeout},
		},
		QueryResponse: steps,
	}
}

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestQueryResponse_Success(t *testing.T) {
	testCases := []struct {
		name    string
		handler func(conn net.Conn)
		steps   []QueryResponseStep
	}{
		{
			name: "banner only",
			handler: func(conn net.Conn) {
				_, _ = conn.Write([]byte("220 mail.example.com ESMTP ready\r\n"))
				_, _ = io.Copy(io.Discard, conn)
			},
			steps: []QueryResponseStep{{Expect: "^220"}},
		},
		{
			name: "banner split across writes",
			handler: func(conn net.Conn) {
				_, _ = conn.Write([]byte("22"))
				time.Sleep(50 * time.Millisecond)
				_, _ = conn.Write([]byte("0 ready\r\n"))
				_, _ = io.Copy(io.Discard, conn)
			},
			steps: []QueryResponseStep{{Expect: "^220 ready"}},
		},
		{
			name: "send then expect",
			handler: func(conn net.Conn) {
				line, err := bufio.NewReader(conn).ReadString('\n')
				if err != nil || line != "PING\r\n" {
					_, _ = conn.Write([]byte("-ERR\r\n"))
					return
				}
				_, _ = conn.Write([]byte("+PONG\r\n"))
			},
			steps: []QueryResponseStep{{Send: "PING\r\n"}, {Expect: `^\+PONG`}},
		},
		{
			name: "multi-step with capture group expansion",
			handler: func(conn net.Conn) {
				_, _ = conn.Write([]byte(":server NOTICE AUTH :*** Looking up your hostname\r\nPING :abc123\r\n"))
				reader := bufio.NewReader(conn)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if line == "PONG abc123\r\n" {
						_, _ = conn.Write([]byte(":server 001 prober :Welcome\r\n"))
						return
					}
				}
			},
			steps: []QueryResponseStep{
				{Send: "NICK prober\r\n"},
				{Expect: `PING :([^ \r\n]+)`, Send: "PONG ${1}\r\n"},
				{Expect: `(?m)^:[^ ]+ 001`},
			},
		},
		{
			name: "binary send and expect",
			handler: func(conn net.Conn) {
				buf := make([]byte, 3)
				if _, err := io.ReadFull(conn, buf); err != nil || !bytes.Equal(buf, []byte{1, 2, 3}) {
					return
				}
				_, _ = conn.Write([]byte{0, 4, 5, 6, 0})
			},
			steps: []QueryResponseStep{{SendBytes: "AQID"}, {ExpectBytes: "BAUG"}},
		},
		{
			name: "data after a match remains available to the next step",
			handler: func(conn net.Conn) {
				_, _ = conn.Write([]byte("A\nB\n"))
				_, _ = io.Copy(io.Discard, conn)
			},
			steps: []QueryResponseStep{{Expect: "A\n"}, {Expect: "^B"}},
		},
		{
			name: "standalone send is written literally",
			handler: func(conn net.Conn) {
				line, err := bufio.NewReader(conn).ReadString('\n')
				if err != nil || line != "cost $1\n" {
					return
				}
				_, _ = conn.Write([]byte("ok\n"))
			},
			steps: []QueryResponseStep{{Send: "cost $1\n"}, {Expect: "^ok"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := startServer(t, tc.handler)
			result := runCheck(t.Context(), t, newTarget(endpoint, 3*time.Second, tc.steps...))
			require.NoError(t, result.err)
			assert.Equal(t, int64(1), result.status)
			assert.Empty(t, result.errorCode)
		})
	}
}

func TestQueryResponse_Failure(t *testing.T) {
	testCases := []struct {
		name             string
		handler          func(conn net.Conn)
		steps            []QueryResponseStep
		maxResponseBytes int
		timeout          time.Duration
		expectedStep     string
		expectedErr      string
	}{
		{
			name: "connection closed before match",
			handler: func(conn net.Conn) {
				_, _ = conn.Write([]byte("500 go away\r\n"))
			},
			steps:        []QueryResponseStep{{Expect: "^220"}},
			timeout:      3 * time.Second,
			expectedStep: "query_response step 0 failed",
			expectedErr:  errConnectionClosed.Error(),
		},
		{
			name: "bytes mismatch before close",
			handler: func(conn net.Conn) {
				_, _ = conn.Write([]byte{9, 9, 9})
			},
			steps:        []QueryResponseStep{{ExpectBytes: b64("\x04\x05\x06")}},
			timeout:      3 * time.Second,
			expectedStep: "query_response step 0 failed",
			expectedErr:  errConnectionClosed.Error(),
		},
		{
			name: "silent server times out",
			handler: func(conn net.Conn) {
				_, _ = io.Copy(io.Discard, conn)
			},
			steps:        []QueryResponseStep{{Send: "PING\r\n"}, {Expect: `^\+PONG`}},
			timeout:      300 * time.Millisecond,
			expectedStep: "query_response step 1 failed",
			expectedErr:  "i/o timeout",
		},
		{
			name: "response exceeds max_response_bytes",
			handler: func(conn net.Conn) {
				filler := bytes.Repeat([]byte("x"), 1024)
				for range 16 {
					if _, err := conn.Write(filler); err != nil {
						return
					}
				}
				_, _ = io.Copy(io.Discard, conn)
			},
			steps:            []QueryResponseStep{{Expect: "^220"}},
			maxResponseBytes: 2048,
			timeout:          3 * time.Second,
			expectedStep:     "query_response step 0 failed",
			expectedErr:      errResponseTooLarge.Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := startServer(t, tc.handler)
			target := newTarget(endpoint, tc.timeout, tc.steps...)
			target.MaxResponseBytes = tc.maxResponseBytes

			start := time.Now()
			result := runCheck(t.Context(), t, target)
			require.Error(t, result.err)
			// The scrape error is flattened by scrapererror, so match on text.
			require.ErrorContains(t, result.err, tc.expectedStep)
			require.ErrorContains(t, result.err, tc.expectedErr)
			require.Less(t, time.Since(start), 3*time.Second, "check should be bounded by the dialer timeout")
			assert.Equal(t, int64(0), result.status)
			assert.Equal(t, "query_response_failed", result.errorCode)
		})
	}
}

func TestQueryResponse_ContextDeadlineBoundsExchange(t *testing.T) {
	endpoint := startServer(t, func(conn net.Conn) {
		_, _ = io.Copy(io.Discard, conn)
	})

	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	// No dialer timeout: only the scrape context bounds the exchange. Once the
	// context is done the scrape error itself is dropped, but the failure is
	// still recorded in the metrics.
	result := runCheck(ctx, t, newTarget(endpoint, 0, QueryResponseStep{Expect: "^220"}))
	require.Less(t, time.Since(start), 3*time.Second)
	assert.Equal(t, int64(0), result.status)
	assert.Equal(t, "query_response_failed", result.errorCode)
}

func TestQueryResponse_ContextCancelUnblocksRead(t *testing.T) {
	endpoint := startServer(t, func(conn net.Conn) {
		_, _ = io.Copy(io.Discard, conn)
	})

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	result := runCheck(ctx, t, newTarget(endpoint, 0, QueryResponseStep{Expect: "^220"}))
	require.Less(t, time.Since(start), 3*time.Second)
	assert.Equal(t, int64(0), result.status)
	assert.Equal(t, "query_response_failed", result.errorCode)
}

func TestQueryResponse_DialOnlyWhenNoSteps(t *testing.T) {
	var sawRead atomic.Bool
	endpoint := startServer(t, func(conn net.Conn) {
		buf := make([]byte, 1)
		if n, _ := conn.Read(buf); n > 0 {
			sawRead.Store(true)
		}
	})

	result := runCheck(t.Context(), t, newTarget(endpoint, time.Second))
	require.NoError(t, result.err)
	assert.Equal(t, int64(1), result.status)
	assert.False(t, sawRead.Load())
}

func TestExchangeDeadline(t *testing.T) {
	start := time.Now()

	t.Run("no deadline", func(t *testing.T) {
		assert.True(t, exchangeDeadline(t.Context(), start, 0).IsZero())
	})
	t.Run("dialer timeout only", func(t *testing.T) {
		assert.Equal(t, start.Add(time.Second), exchangeDeadline(t.Context(), start, time.Second))
	})
	t.Run("context deadline only", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(t.Context(), start.Add(time.Minute))
		defer cancel()
		assert.Equal(t, start.Add(time.Minute), exchangeDeadline(ctx, start, 0))
	})
	t.Run("earliest wins", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(t.Context(), start.Add(time.Minute))
		defer cancel()
		assert.Equal(t, start.Add(time.Second), exchangeDeadline(ctx, start, time.Second))
		assert.Equal(t, start.Add(time.Minute), exchangeDeadline(ctx, start, time.Hour))
	})
}

func TestErrorCodeFor(t *testing.T) {
	assert.Equal(t, metadata.AttributeErrorCodeQueryResponseFailed,
		errorCodeFor(&queryResponseError{step: 1, err: errors.New("read tcp: i/o timeout")}))
	assert.Equal(t, metadata.AttributeErrorCodeConnectionTimeout,
		errorCodeFor(errors.New("dial tcp: i/o timeout")))
	assert.Equal(t, metadata.AttributeErrorCodeConnectionRefused,
		errorCodeFor(errors.New("dial tcp: connection refused")))
	assert.Equal(t, metadata.AttributeErrorCodeUnknownError,
		errorCodeFor(errors.New("something else")))
}

func TestQueryResponseError_DoesNotLeakResponse(t *testing.T) {
	endpoint := startServer(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte("secret-token-12345\r\n"))
	})

	result := runCheck(t.Context(), t, newTarget(endpoint, time.Second, QueryResponseStep{Expect: "^220"}))
	require.Error(t, result.err)
	assert.NotContains(t, result.err.Error(), "secret-token")
	assert.NotContains(t, result.err.Error(), "^220")
}
