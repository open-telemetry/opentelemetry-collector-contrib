// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmiddleware"
	"go.opentelemetry.io/collector/extension/extensionmiddleware"
)

func TestRetry_DNSDeadlineExceeded(t *testing.T) {
	// A deadline on the dial context during lookup is not a bare
	// context.DeadlineExceeded. net.Dialer returns:
	//   *net.OpError{Op: "dial"} -> *net.DNSError -> timeout that matches
	//   context.DeadlineExceeded
	// ("dial tcp: lookup <host>: i/o timeout"). RetryOnError must retry that.
	// The resolver blocks until that dial deadline so the test does not depend
	// on a real DNS server.
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	dialer := &net.Dialer{
		Timeout:  50 * time.Millisecond,
		Resolver: resolver,
	}
	const maxRetries = 1
	client, dials := newRetryClient(t, "http://es.invalid:9200", time.Second, dialer.DialContext)

	req, err := http.NewRequest(http.MethodPost, "http://es.invalid:9200/_bulk", http.NoBody)
	require.NoError(t, err)
	_, err = client.Perform(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var setupErr *connSetupTimeoutError
	require.ErrorAs(t, err, &setupErr)
	var dnsErr *net.DNSError
	require.ErrorAs(t, err, &dnsErr)
	var opErr *net.OpError
	require.ErrorAs(t, err, &opErr)
	assert.Equal(t, "dial", opErr.Op)
	assert.Equal(t, int32(maxRetries+1), dials.Load())
}

func TestRetry_ResponseDeadlineExceeded_NoRetry(t *testing.T) {
	// The handshake completes. The server accepts the request and writes no
	// response. The client deadline is context.DeadlineExceeded from waiting
	// on that response, so it must not be retried.
	release := make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
		<-release
	}))
	t.Cleanup(func() {
		close(release)
		server.Close()
	})

	client, dials := newRetryClient(t, server.URL, 50*time.Millisecond, (&net.Dialer{}).DialContext)

	req, err := http.NewRequest(http.MethodPost, server.URL+"/_bulk", http.NoBody)
	require.NoError(t, err)
	_, err = client.Perform(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var setupErr *connSetupTimeoutError
	require.NotErrorAs(t, err, &setupErr)
	var opErr *net.OpError
	require.False(t, errors.As(err, &opErr) && opErr.Op == "dial")
	assert.Equal(t, int32(1), dials.Load())
	assert.Equal(t, int32(1), requests.Load())
}

func TestRetry_RequestDeadlineDuringDial(t *testing.T) {
	// Dial has no deadline of its own and outlasts the request timeout.
	// Transport.getConn dials on a context detached from the request, so this
	// dial keeps running after the deadline. getConn then returns a bare
	// context.DeadlineExceeded and drops the dial error. That behavior is
	// specific to net/http. A second dial and a connSetupTimeoutError show that the
	// deadline was classified as connection setup: a bare context.DeadlineExceeded
	// is not retried. errors.Is still matches the transport error through Unwrap.
	const maxRetries = 1
	var finished atomic.Int32
	client, dials := newRetryClient(t, "http://es.invalid:9200", 50*time.Millisecond, func(ctx context.Context, _, _ string) (net.Conn, error) {
		defer finished.Add(1)
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, context.DeadlineExceeded
		}
	})
	t.Cleanup(func() {
		require.Eventually(t, func() bool {
			return dials.Load() > maxRetries && finished.Load() == dials.Load()
		}, 3*time.Second, 10*time.Millisecond)
	})

	req, err := http.NewRequest(http.MethodPost, "http://es.invalid:9200/_bulk", http.NoBody)
	require.NoError(t, err)
	_, err = client.Perform(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var setupErr *connSetupTimeoutError
	require.ErrorAs(t, err, &setupErr)
	var dnsErr *net.DNSError
	require.NotErrorAs(t, err, &dnsErr)
	assert.Equal(t, int32(maxRetries+1), dials.Load())
}

func TestRetry_DeadlineBeforeConnectionReady(t *testing.T) {
	// The interceptor classifies the deadline. A bare deadline, including one
	// that wraps a read after the connection exists, is not retried.
	assert.False(t, retryOnError(nil, context.DeadlineExceeded))
	assert.False(t, retryOnError(nil, context.Canceled))
	assert.False(t, retryOnError(nil, &net.OpError{Op: "read", Net: "tcp", Err: context.DeadlineExceeded}))
	assert.True(t, retryOnError(nil, &connSetupTimeoutError{err: context.DeadlineExceeded}))
	assert.True(t, retryOnError(nil, errors.New("connection reset")))
}

func TestRetry_WriteDeadlineExceeded_NoRetry(t *testing.T) {
	// Dial returns a connection, so GotConn has fired, and the write then blocks.
	// A deadline while the request is being written must not be retried.
	serverConns := make(chan net.Conn, 2)
	client, dials := newRetryClient(t, "http://es.invalid:9200", 50*time.Millisecond, func(context.Context, string, string) (net.Conn, error) {
		clientConn, serverConn := net.Pipe()
		serverConns <- serverConn
		return clientConn, nil
	})
	t.Cleanup(func() {
		close(serverConns)
		for conn := range serverConns {
			_ = conn.Close()
		}
	})

	payload := bytes.Repeat([]byte("{\"index\":{}}\n"), 1024)
	req, err := http.NewRequest(http.MethodPost, "http://es.invalid:9200/_bulk", bytes.NewReader(payload))
	require.NoError(t, err)
	_, err = client.Perform(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var setupErr *connSetupTimeoutError
	require.NotErrorAs(t, err, &setupErr)
	assert.Equal(t, int32(1), dials.Load())
}

func TestRetry_CallerDeadlineDuringDial_NoRetry(t *testing.T) {
	// The caller's context expires during the dial. That deadline is not a
	// per-attempt setup timeout, so it is not wrapped and not retried.
	var finished atomic.Int32
	client, dials := newRetryClient(t, "http://es.invalid:9200", time.Second, func(ctx context.Context, _, _ string) (net.Conn, error) {
		defer finished.Add(1)
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, context.DeadlineExceeded
		}
	})
	t.Cleanup(func() {
		require.Eventually(t, func() bool {
			return finished.Load() == dials.Load() && dials.Load() > 0
		}, 3*time.Second, 10*time.Millisecond)
	})

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://es.invalid:9200/_bulk", http.NoBody)
	require.NoError(t, err)
	_, err = client.Perform(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var setupErr *connSetupTimeoutError
	require.NotErrorAs(t, err, &setupErr)
	assert.Equal(t, int32(1), dials.Load())
}

// newRetryClient builds a client with newElasticsearchClient. dial is installed
// on the HTTP transport confighttp creates, and each dial is counted.
func newRetryClient(t *testing.T, endpoint string, clientTimeout time.Duration, dial func(context.Context, string, string) (net.Conn, error)) (elastictransport.Interface, *atomic.Int32) {
	t.Helper()
	dials := &atomic.Int32{}
	id := component.MustNewID("dialctx")
	mw := dialCtxMiddleware{
		GetHTTPRoundTripperFunc: func(context.Context) (extensionmiddleware.WrapHTTPRoundTripperFunc, error) {
			return func(_ context.Context, rt http.RoundTripper) (http.RoundTripper, error) {
				tr, ok := rt.(*http.Transport)
				if !ok {
					return nil, fmt.Errorf("elasticsearch HTTP transport is %T, want *http.Transport", rt)
				}
				tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
					dials.Add(1)
					return dial(ctx, network, addr)
				}
				return tr, nil
			}, nil
		},
	}
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoints = []string{endpoint}
	cfg.ClientConfig.Timeout = clientTimeout
	cfg.Retry.MaxRetries = 1
	cfg.Retry.InitialInterval = time.Millisecond
	cfg.Retry.MaxInterval = time.Millisecond
	cfg.ClientConfig.Middlewares = []configmiddleware.Config{{ID: id}}

	client, err := newElasticsearchClient(t.Context(), cfg, dialHost{extensions: map[component.ID]component.Component{id: mw}}, componenttest.NewNopTelemetrySettings(), "")
	require.NoError(t, err)
	return client, dials
}

// dialCtxMiddleware sets DialContext on the *http.Transport created for the exporter.
type dialCtxMiddleware struct {
	extensionmiddleware.GetHTTPRoundTripperFunc
}

func (dialCtxMiddleware) Start(context.Context, component.Host) error { return nil }
func (dialCtxMiddleware) Shutdown(context.Context) error              { return nil }

type dialHost struct {
	extensions map[component.ID]component.Component
}

func (h dialHost) GetExtensions() map[component.ID]component.Component { return h.extensions }
