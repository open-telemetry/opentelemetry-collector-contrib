// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
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
	// The resolver blocks until the dial deadline, so the test does not use a real DNS server.
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
	assert.Equal(t, int32(1), dials.Load())
	assert.Equal(t, int32(1), requests.Load())
}

func TestRetry_RequestDeadlineDuringDial(t *testing.T) {
	// getConn dials on a context detached from the request, so the dial keeps running after the deadline.
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
	// The dial keeps running after the test returns. Wait for it so goleak passes.
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
	assert.Equal(t, int32(maxRetries+1), dials.Load())
}

func TestRetry_TLSHandshakeDeadlineExceeded(t *testing.T) {
	// The server accepts TCP and sends nothing, so the deadline fires during
	// the TLS handshake, before GotConn.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var mu sync.Mutex
	var accepted []net.Conn
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			conn, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			mu.Lock()
			accepted = append(accepted, conn)
			mu.Unlock()
		}
	})
	t.Cleanup(func() {
		_ = ln.Close()
		wg.Wait()
		mu.Lock()
		defer mu.Unlock()
		for _, conn := range accepted {
			_ = conn.Close()
		}
	})

	endpoint := "https://" + ln.Addr().String()
	const maxRetries = 1
	client, dials := newRetryClient(t, endpoint, 50*time.Millisecond, (&net.Dialer{}).DialContext)

	req, err := http.NewRequest(http.MethodPost, endpoint+"/_bulk", http.NoBody)
	require.NoError(t, err)
	_, err = client.Perform(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	var setupErr *connSetupTimeoutError
	require.ErrorAs(t, err, &setupErr)
	assert.Equal(t, int32(maxRetries+1), dials.Load())
}

func TestRetry_WriteDeadlineExceeded_NoRetry(t *testing.T) {
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

	// Larger than the transport's 4KiB write buffer, so the deadline fires inside
	// Request.write, before WroteRequest.
	payload := bytes.Repeat([]byte("{\"index\":{}}\n"), 1024)
	req, err := http.NewRequest(http.MethodPost, "http://es.invalid:9200/_bulk", bytes.NewReader(payload))
	require.NoError(t, err)
	_, err = client.Perform(req)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, int32(1), dials.Load())
}

func TestRetry_CallerDeadlineDuringDial_NoRetry(t *testing.T) {
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
	// The dial keeps running after the test returns. Wait for it so goleak passes.
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
