// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"fmt"
	"net"
	"net/http"
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

	// Shorter than the 1s client timeout and the dial's 1s fallback, so this
	// deadline is what ends the request. Long enough for the dial goroutine to
	// enter DialContext before net/http abandons the dial.
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
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
