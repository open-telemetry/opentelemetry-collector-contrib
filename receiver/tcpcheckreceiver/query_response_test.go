// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/metadata"
)

func TestQueryResponseValidation(t *testing.T) {
	tests := []struct {
		name string
		step QueryResponse
		err  string
	}{
		{name: "send", step: QueryResponse{Send: "PING\r\n"}},
		{name: "expect", step: QueryResponse{Expect: "^220"}},
		{name: "binary", step: QueryResponse{SendBytes: "AAE=", ExpectBytes: "AgM="}},
		{name: "capture", step: QueryResponse{Expect: "PING (.+)", Send: "PONG ${1}"}},
		{name: "empty", err: "at least one operation"},
		{name: "conflicting sends", step: QueryResponse{Send: "a", SendBytes: "YQ=="}, err: "send and send_bytes are mutually exclusive"},
		{name: "conflicting expects", step: QueryResponse{Expect: "a", ExpectBytes: "YQ=="}, err: "expect and expect_bytes are mutually exclusive"},
		{name: "invalid regex", step: QueryResponse{Expect: "["}, err: "invalid expect regular expression"},
		{name: "invalid send base64", step: QueryResponse{SendBytes: "!"}, err: "invalid send_bytes base64"},
		{name: "invalid expect base64", step: QueryResponse{ExpectBytes: "!"}, err: "invalid expect_bytes base64"},
		{name: "empty decoded expectation", step: QueryResponse{ExpectBytes: "\n"}, err: "at least one byte"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Targets: []*TargetConfig{{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint:     "localhost:1234",
					DialerConfig: confignet.DialerConfig{Timeout: time.Second},
				},
				QueryResponse: []QueryResponse{tt.step},
			}}}
			err := cfg.Validate()
			if tt.err != "" {
				require.ErrorContains(t, err, "query_response[0]")
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			step := cfg.Targets[0].QueryResponse[0]
			if step.Expect != "" {
				require.NotNil(t, step.expectRegexp)
			}
			if step.SendBytes != "" {
				assert.Equal(t, []byte{0, 1}, step.sendData)
				assert.Equal(t, []byte{2, 3}, step.expectData)
			}
		})
	}
}

func TestQueryResponseTargetValidation(t *testing.T) {
	tests := []struct {
		name   string
		target *TargetConfig
		err    string
	}{
		{name: "null target", err: "target must not be null"},
		{name: "dial only", target: &TargetConfig{}},
		{name: "negative buffer", target: &TargetConfig{MaxReadSize: -1}, err: "max_read_size must not be negative"},
		{name: "missing timeout", target: &TargetConfig{QueryResponse: []QueryResponse{{Expect: "OK"}}}, err: "dialer.timeout must be positive"},
		{name: "negative timeout", target: &TargetConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{DialerConfig: confignet.DialerConfig{Timeout: -time.Second}},
			QueryResponse: []QueryResponse{{Expect: "OK"}},
		}, err: "dialer.timeout must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.target != nil {
				tt.target.Endpoint = "localhost:1234"
			}
			cfg := &Config{Targets: []*TargetConfig{tt.target}}
			err := cfg.Validate()
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadQueryResponseConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf("testdata/query_response.yaml")
	require.NoError(t, err)
	cfg := newDefaultConfig().(*Config)
	require.NoError(t, cm.Unmarshal(cfg))
	require.NoError(t, cfg.Validate())
	require.Len(t, cfg.Targets, 4)
	assert.Equal(t, 1024, cfg.Targets[0].MaxReadSize)
	assert.Equal(t, "mail.example.com:25", cfg.Targets[0].Endpoint)
	assert.Equal(t, 5*time.Second, cfg.Targets[0].DialerConfig.Timeout)
	require.Len(t, cfg.Targets[0].QueryResponse, 1)
	assert.Equal(t, "^220", cfg.Targets[0].QueryResponse[0].Expect)
	assert.Equal(t, "localhost:6379", cfg.Targets[1].Endpoint)
	require.Len(t, cfg.Targets[1].QueryResponse, 2)
	assert.Equal(t, "PING\r\n", cfg.Targets[1].QueryResponse[0].Send)
	assert.Equal(t, `^\+PONG\r\n`, cfg.Targets[1].QueryResponse[1].Expect)
	assert.Equal(t, "PONG ${1}\r\n", cfg.Targets[2].QueryResponse[0].Send)
	assert.Equal(t, []byte{0, 1}, cfg.Targets[3].QueryResponse[0].sendData)
	assert.Equal(t, []byte{2, 3}, cfg.Targets[3].QueryResponse[1].expectData)
}

func TestQueryResponseMetricsGolden(t *testing.T) {
	cm, err := confmaptest.LoadConf("testdata/query_response.yaml")
	require.NoError(t, err)
	cfg := newDefaultConfig().(*Config)
	require.NoError(t, cm.Unmarshal(cfg))
	// The golden metrics cover the expect-only and send-then-expect examples.
	cfg.Targets = cfg.Targets[:2]
	handlers := []func(net.Conn) error{
		func(conn net.Conn) error {
			_, writeErr := io.WriteString(conn, "220 ready\r\n")
			return writeErr
		},
		func(conn net.Conn) error {
			if readErr := readQueryResponseRequest(conn, "PING\r\n"); readErr != nil {
				return readErr
			}
			_, writeErr := io.WriteString(conn, "+PONG\r\n")
			return writeErr
		},
	}
	endpoints := make(map[string]string, len(handlers))
	for i, handler := range handlers {
		target := cfg.Targets[i]
		endpoint := startQueryResponseServer(t, handler)
		endpoints[target.Endpoint] = endpoint
		target.Endpoint = endpoint
	}
	require.NoError(t, cfg.Validate())

	expected, err := golden.ReadMetrics("testdata/expected_metrics/metrics_golden.yaml")
	require.NoError(t, err)
	// Replace the example endpoints with the local test servers' assigned ports.
	metrics := expected.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := range metrics.Len() {
		points := metrics.At(i).Gauge().DataPoints()
		for j := range points.Len() {
			attrs := points.At(j).Attributes()
			endpoint, ok := attrs.Get("tcpcheck.endpoint")
			require.True(t, ok)
			actualEndpoint, ok := endpoints[endpoint.Str()]
			require.True(t, ok, "unexpected endpoint in golden metrics: %s", endpoint.Str())
			attrs.PutStr("tcpcheck.endpoint", actualEndpoint)
		}
	}

	s := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	actual, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, actual,
		pmetrictest.IgnoreMetricValues("tcpcheck.duration"),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestResponseReader(t *testing.T) {
	tests := []struct {
		name     string
		response string
		step     QueryResponse
		limit    int
		wantSend string
		wantErr  string
	}{
		{name: "fragmented regex", response: "220 ready\r\n", step: QueryResponse{Expect: "^220 ready\\r\\n"}, limit: 64},
		{name: "fragmented binary", response: "\xff\x00\x01\x02", step: QueryResponse{ExpectBytes: "AAEC"}, limit: 64},
		{name: "capture expansion", response: "PING :token\r\n", step: QueryResponse{Expect: "PING :([^\\r\\n]+)\\r\\n", Send: "PONG ${1}\r\n"}, limit: 64, wantSend: "PONG token\r\n"},
		{name: "exact limit", response: "PONG", step: QueryResponse{Expect: "PONG"}, limit: 4},
		{name: "buffer limit", response: "secret", step: QueryResponse{Expect: "private"}, limit: 4, wantErr: "max_read_size exceeded"},
		{name: "EOF", response: "secret", step: QueryResponse{Expect: "private"}, limit: 64, wantErr: "response did not match expectation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.step.validate())
			r := responseReader{reader: iotest.OneByteReader(strings.NewReader(tt.response)), maxReadSize: tt.limit}
			send, err := r.expect(&tt.step)
			if tt.wantErr != "" {
				require.ErrorIs(t, err, errExpectMismatch)
				require.ErrorContains(t, err, tt.wantErr)
				assert.NotContains(t, err.Error(), "secret")
				assert.NotContains(t, err.Error(), "private")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSend, string(send))
		})
	}
}

func TestResponseReaderPreservesReadAhead(t *testing.T) {
	r := responseReader{reader: iotest.DataErrReader(strings.NewReader("first\r\nsecond\r\n")), maxReadSize: 64}
	for _, pattern := range []string{"^first\\r\\n", "^second\\r\\n"} {
		step := QueryResponse{Expect: pattern}
		require.NoError(t, step.validate())
		_, err := r.expect(&step)
		require.NoError(t, err)
	}
	step := QueryResponse{Expect: "third"}
	require.NoError(t, step.validate())
	_, err := r.expect(&step)
	require.ErrorIs(t, err, errExpectMismatch)
}

// startQueryResponseServer serves one check and reports handler errors during cleanup.
func startQueryResponseServer(t *testing.T, handler func(net.Conn) error) string {
	t.Helper()
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })
	// Bound Accept if the test fails before connecting to the server.
	require.NoError(t, listener.SetDeadline(time.Now().Add(5*time.Second)))
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			done <- err
			return
		}
		done <- handler(conn)
	}()
	t.Cleanup(func() {
		// Cleanups run in reverse order: wait for Accept and the handler before
		// closing the listener, including when a dial-only check finishes first.
		assert.NoError(t, <-done)
	})
	return listener.Addr().String()
}

func readQueryResponseRequest(conn net.Conn, expected string) error {
	request := make([]byte, len(expected))
	if _, err := io.ReadFull(conn, request); err != nil {
		return err
	}
	if string(request) != expected {
		return errors.New("unexpected request bytes")
	}
	return nil
}

func TestQueryResponseChecks(t *testing.T) {
	tests := []struct {
		name    string
		steps   []QueryResponse
		handler func(net.Conn) error
	}{
		{name: "dial only", handler: func(conn net.Conn) error {
			_, err := io.Copy(io.Discard, conn)
			return err
		}},
		{name: "banner", steps: []QueryResponse{{Expect: "^220"}}, handler: func(conn net.Conn) error {
			_, err := io.WriteString(conn, "220 ready\r\n")
			return err
		}},
		{name: "send only", steps: []QueryResponse{{Send: "QUIT\r\n"}}, handler: func(conn net.Conn) error {
			return readQueryResponseRequest(conn, "QUIT\r\n")
		}},
		{name: "redis", steps: []QueryResponse{{Send: "PING\r\n"}, {Expect: "^\\+PONG\\r\\n"}}, handler: func(conn net.Conn) error {
			if err := readQueryResponseRequest(conn, "PING\r\n"); err != nil {
				return err
			}
			_, err := io.WriteString(conn, "+PONG\r\n")
			return err
		}},
		{name: "capture and read ahead", steps: []QueryResponse{
			{Expect: "PING :([^\\r\\n]+)\\r\\n", Send: "PONG ${1}\r\n"},
			{Expect: "^OK\\r\\n"},
		}, handler: func(conn net.Conn) error {
			if _, err := io.WriteString(conn, "PING :token\r\nOK\r\n"); err != nil {
				return err
			}
			return readQueryResponseRequest(conn, "PONG token\r\n")
		}},
		{name: "binary", steps: []QueryResponse{{SendBytes: "AAE="}, {ExpectBytes: "AgM=", SendBytes: "BAU="}}, handler: func(conn net.Conn) error {
			if err := readQueryResponseRequest(conn, "\x00\x01"); err != nil {
				return err
			}
			if _, err := conn.Write([]byte{2, 3}); err != nil {
				return err
			}
			return readQueryResponseRequest(conn, "\x04\x05")
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &TargetConfig{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint:     startQueryResponseServer(t, tt.handler),
					DialerConfig: confignet.DialerConfig{Timeout: time.Second},
				},
				QueryResponse: tt.steps,
			}
			cfg := &Config{Targets: []*TargetConfig{target}}
			require.NoError(t, cfg.Validate())
			s := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
			metrics, err := s.scrape(t.Context())
			require.NoError(t, err)
			ms := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			require.Equal(t, 2, ms.Len())
			for i := range ms.Len() {
				metric := ms.At(i)
				if metric.Name() == "tcpcheck.status" {
					assert.Equal(t, int64(1), metric.Gauge().DataPoints().At(0).IntValue())
				} else {
					assert.Equal(t, "tcpcheck.duration", metric.Name())
				}
			}
		})
	}
}

func TestQueryResponseFailureMetrics(t *testing.T) {
	tests := []struct {
		name    string
		limit   int
		code    string
		handler func(net.Conn) error
	}{
		{name: "mismatch", code: "expect_mismatch", handler: func(conn net.Conn) error {
			if err := readQueryResponseRequest(conn, "PING\r\n"); err != nil {
				return err
			}
			_, err := io.WriteString(conn, "secret")
			return err
		}},
		{name: "buffer limit", limit: 4, code: "expect_mismatch", handler: func(conn net.Conn) error {
			if err := readQueryResponseRequest(conn, "PING\r\n"); err != nil {
				return err
			}
			_, err := io.WriteString(conn, "secret")
			return err
		}},
		{name: "default buffer limit", code: "expect_mismatch", handler: func(conn net.Conn) error {
			if err := readQueryResponseRequest(conn, "PING\r\n"); err != nil {
				return err
			}
			_, err := io.WriteString(conn, strings.Repeat("x", defaultMaxReadSize+1))
			return err
		}},
		{name: "timeout", code: "step_timeout", handler: func(conn net.Conn) error {
			_, err := io.Copy(io.Discard, conn)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &TargetConfig{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint:     startQueryResponseServer(t, tt.handler),
					DialerConfig: confignet.DialerConfig{Timeout: 100 * time.Millisecond},
				},
				MaxReadSize:   tt.limit,
				QueryResponse: []QueryResponse{{Send: "PING\r\n"}, {Expect: "private", Send: "must not be sent"}},
			}
			cfg := &Config{Targets: []*TargetConfig{target}}
			require.NoError(t, cfg.Validate())
			s := newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
			metrics, err := s.scrape(t.Context())
			require.ErrorContains(t, err, "query_response[1]")
			assert.NotContains(t, err.Error(), "secret")
			assert.NotContains(t, err.Error(), "private")
			ms := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			require.Equal(t, 2, ms.Len())
			for i := range ms.Len() {
				metric := ms.At(i)
				switch metric.Name() {
				case "tcpcheck.error":
					point := metric.Sum().DataPoints().At(0)
					code, ok := point.Attributes().Get("error.code")
					require.True(t, ok)
					assert.Equal(t, tt.code, code.Str())
					assert.Equal(t, int64(1), point.IntValue())
					assert.Equal(t, 2, point.Attributes().Len())
				case "tcpcheck.status":
					assert.Zero(t, metric.Gauge().DataPoints().At(0).IntValue())
				default:
					t.Errorf("unexpected metric: %s", metric.Name())
				}
			}
		})
	}
}

func TestQueryResponseCancellation(t *testing.T) {
	connected := make(chan struct{})
	endpoint := startQueryResponseServer(t, func(conn net.Conn) error {
		close(connected)
		_, err := io.Copy(io.Discard, conn)
		return err
	})
	target := &TargetConfig{
		TCPAddrConfig: confignet.TCPAddrConfig{
			Endpoint:     endpoint,
			DialerConfig: confignet.DialerConfig{Timeout: time.Hour},
		},
		QueryResponse: []QueryResponse{{Expect: "OK"}},
	}
	require.NoError(t, target.validateQueryResponse())
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := getConnectionState(ctx, target)
		done <- err
	}()
	<-connected
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("check did not stop after context cancellation")
	}
}

func TestQueryResponseWriteFailure(t *testing.T) {
	client, server := net.Pipe()
	server.Close()
	t.Cleanup(func() { client.Close() })
	target := &TargetConfig{QueryResponse: []QueryResponse{{Send: "PING"}}}
	err := runQueryResponse(t.Context(), client, target)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.ErrorContains(t, err, "query_response[0]")
}

func TestQueryResponseSharedDeadline(t *testing.T) {
	endpoint := startQueryResponseServer(t, func(conn net.Conn) error {
		request := make([]byte, 1)
		for range 3 {
			if _, err := io.ReadFull(conn, request); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
			time.Sleep(60 * time.Millisecond)
			if _, err := io.WriteString(conn, "OK"); err != nil {
				return nil // The client closes when the shared deadline expires.
			}
		}
		return nil
	})
	target := &TargetConfig{
		TCPAddrConfig: confignet.TCPAddrConfig{
			Endpoint:     endpoint,
			DialerConfig: confignet.DialerConfig{Timeout: 150 * time.Millisecond},
		},
		QueryResponse: []QueryResponse{{Send: "?"}, {Expect: "OK", Send: "?"}, {Expect: "OK", Send: "?"}, {Expect: "OK"}},
	}
	require.NoError(t, target.validateQueryResponse())
	start := time.Now()
	_, err := getConnectionState(t.Context(), target)
	require.ErrorContains(t, err, "query_response[")
	var timeoutErr net.Error
	require.True(t, errors.As(err, &timeoutErr) && timeoutErr.Timeout())
	assert.Less(t, time.Since(start), time.Second)
}
