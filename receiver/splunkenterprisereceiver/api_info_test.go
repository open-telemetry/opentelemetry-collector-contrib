// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

func newAPIInfoScraper(t *testing.T, client *http.Client, endpoint string) *splunkScraper {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	cfg.MetricsBuilderConfig.Metrics.SplunkHealth.Enabled = false
	cfg.MetricsBuilderConfig.Metrics.SplunkAPIInfoUp.Enabled = true
	s := newSplunkMetricsScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	s.splunkClient = &splunkEntClient{clients: splunkClientMap{
		typeIdx: {client: client, endpoint: u},
	}}
	return &s
}

func requireAPIInfoUp(t *testing.T, metrics pmetric.Metrics, endpoint string, value int64) {
	t.Helper()
	require.Equal(t, 1, metrics.DataPointCount())
	metric := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	require.Equal(t, "splunk.api.info.up", metric.Name())
	require.Equal(t, "1", metric.Unit())
	require.Equal(t, pmetric.MetricTypeGauge, metric.Type())
	dp := metric.Gauge().DataPoints().At(0)
	require.Equal(t, pmetric.NumberDataPointValueTypeInt, dp.ValueType())
	require.Equal(t, value, dp.IntValue())
	require.NotZero(t, dp.Timestamp())
	host, ok := dp.Attributes().Get("splunk.host")
	require.True(t, ok)
	require.Equal(t, endpoint, host.Str())
}

func TestScrapeAPIInfoUp(t *testing.T) {
	for _, status := range []int{200, 204, 302, 401, 403, 404, 500, 503} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/services/server/info", r.URL.Path)
				assert.Equal(t, "json", r.URL.Query().Get("output_mode"))
				w.Header().Set("Location", "/redirect")
				w.WriteHeader(status)
				// Availability depends on the HTTP status, not JSON decoding.
				_, _ = io.WriteString(w, "not JSON")
			}))
			defer server.Close()
			s := newAPIInfoScraper(t, server.Client(), server.URL)
			metrics, err := s.scrape(t.Context())
			require.NoError(t, err)
			var want int64
			if status == http.StatusOK {
				want = 1
			}
			requireAPIInfoUp(t, metrics, server.URL, want)
		})
	}
}

func TestScrapeAPIInfoUpDisabled(t *testing.T) {
	s := newAPIInfoScraper(t, &http.Client{Transport: apiInfoRoundTripper(func(*http.Request) (*http.Response, error) {
		t.Error("disabled metric must not make a request")
		return nil, errors.New("unexpected request")
	})}, "http://localhost:8089")
	s.conf.MetricsBuilderConfig.Metrics.SplunkAPIInfoUp.Enabled = false
	s.mb = metadata.NewMetricsBuilder(s.conf.MetricsBuilderConfig, receivertest.NewNopSettings(metadata.Type))
	require.False(t, metadata.NewDefaultMetricsBuilderConfig().Metrics.SplunkAPIInfoUp.Enabled)
	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.Zero(t, metrics.DataPointCount())
}

func TestScrapeAPIInfoUpAllEndpoints(t *testing.T) {
	s := newAPIInfoScraper(t, &http.Client{Transport: apiInfoRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})}, "http://indexer:8089")
	for _, endpointType := range []string{typeSh, typeCm} {
		u, err := url.Parse("http://" + endpointType + ":8089")
		require.NoError(t, err)
		s.splunkClient.clients[endpointType] = splunkClient{client: s.splunkClient.clients[typeIdx].client, endpoint: u}
	}
	metrics, err := s.scrape(t.Context())
	require.Error(t, err)
	require.Equal(t, 3, metrics.DataPointCount())
	points := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
	hosts := make(map[string]bool)
	for _, dp := range points.All() {
		require.Zero(t, dp.IntValue())
		host, ok := dp.Attributes().Get("splunk.host")
		require.True(t, ok)
		hosts[host.Str()] = true
	}
	require.Len(t, hosts, 3)
}

func TestScrapeAPIInfoUpCancellation(t *testing.T) {
	s := newAPIInfoScraper(t, &http.Client{Transport: apiInfoRoundTripper(func(req *http.Request) (*http.Response, error) {
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(time.Second):
			t.Error("scrape context was not propagated to the request")
			return nil, errors.New("request did not cancel")
		}
	})}, "http://localhost:8089")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	errs := make(chan error, 1)
	s.scrapeAPIInfoUp(ctx, pcommon.NewTimestampFromTime(time.Now()), nil, errs)
	require.ErrorIs(t, <-errs, context.Canceled)
	requireAPIInfoUp(t, s.mb.Emit(), "http://localhost:8089", 0)
}

func TestScrapeAPIInfoUpClosesBody(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusUnauthorized} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			body := &apiInfoBody{Reader: strings.NewReader(strings.Repeat("response", 1000))}
			s := newAPIInfoScraper(t, &http.Client{Transport: apiInfoRoundTripper(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: status, Body: body, Header: make(http.Header)}, nil
			})}, "http://localhost:8089")
			_, err := s.scrape(t.Context())
			require.NoError(t, err)
			require.True(t, body.closed)
			require.Zero(t, body.Len(), "drain the body to allow connection reuse")
		})
	}
}

func TestScrapeAPIInfoUpRecoveryAndConnectionReuse(t *testing.T) {
	statuses := make(chan int, 2)
	statuses <- http.StatusServiceUnavailable
	statuses <- http.StatusOK
	connections := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		connections <- r.RemoteAddr
		w.WriteHeader(<-statuses)
		_, _ = io.WriteString(w, strings.Repeat("response", 1000))
	}))
	defer server.Close()
	s := newAPIInfoScraper(t, server.Client(), server.URL)
	authID := component.MustNewIDWithName("basicauth", "client")
	s.conf.IdxEndpoint.Endpoint = server.URL
	s.conf.IdxEndpoint.Auth = configoptional.Some(configauth.Config{AuthenticatorID: authID})
	s.conf.IdxEndpoint.Headers = configopaque.MapList{{Name: "Authorization", Value: "Bearer test-token"}}
	require.NoError(t, s.conf.Validate())
	require.NoError(t, s.start(t.Context(), &mockHost{extensions: map[component.ID]component.Component{
		authID: extensionauthtest.NewNopClient(),
	}}))
	for _, want := range []int64{0, 1} {
		metrics, err := s.scrape(t.Context())
		require.NoError(t, err)
		requireAPIInfoUp(t, metrics, server.URL, want)
	}
	firstConnection, secondConnection := <-connections, <-connections
	require.Equal(t, firstConnection, secondConnection, "reuse the connection after draining both success and error bodies")
}

type apiInfoRoundTripper func(*http.Request) (*http.Response, error)

func (rt apiInfoRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt(req)
}

type apiInfoBody struct {
	*strings.Reader
	closed bool
}

func (b *apiInfoBody) Close() error {
	b.closed = true
	return nil
}
