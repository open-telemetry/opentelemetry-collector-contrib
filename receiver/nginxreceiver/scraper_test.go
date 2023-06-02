// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nginxreceiver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestScraper(t *testing.T) {
	nginxMock := newMockServer(t)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = nginxMock.URL + "/status"
	require.NoError(t, component.ValidateConfig(cfg))

	scraper := newNginxScraper(receivertest.NewNopCreateSettings(), cfg)

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp()))
}

func TestScraperWithConnectionsAsSum(t *testing.T) {
	nginxMock := newMockServer(t)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = nginxMock.URL + "/status"
	require.NoError(t, component.ValidateConfig(cfg))

	require.NoError(t, featuregate.GlobalRegistry().Set(connectionsAsSum, true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(connectionsAsSum, false))
	}()

	scraper := newNginxScraper(receivertest.NewNopCreateSettings(), cfg)

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected_with_connections_as_sum.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreMetricsOrder()))
}

func TestScraperError(t *testing.T) {
	nginxMock := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/status" {
			rw.WriteHeader(200)
			_, _ = rw.Write([]byte(`Bad status page`))
			return
		}
		rw.WriteHeader(404)
	}))
	t.Run("404", func(t *testing.T) {
		sc := newNginxScraper(receivertest.NewNopCreateSettings(), &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: nginxMock.URL + "/badpath",
			},
		})
		err := sc.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
		_, err = sc.scrape(context.Background())
		require.Equal(t, errors.New("expected 200 response, got 404"), err)
	})

	t.Run("parse error", func(t *testing.T) {
		sc := newNginxScraper(receivertest.NewNopCreateSettings(), &Config{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: nginxMock.URL + "/status",
			},
		})
		err := sc.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
		_, err = sc.scrape(context.Background())
		require.Equal(t, errors.New("failed to parse response body \"Bad status page\": invalid input \"Bad status page\""), err)
	})
}

func TestScraperFailedStart(t *testing.T) {
	sc := newNginxScraper(receivertest.NewNopCreateSettings(), &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "localhost:8080",
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: "/non/existent",
				},
			},
		},
	})
	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

func newMockServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/status" {
			rw.WriteHeader(200)
			_, err := rw.Write([]byte(`Active connections: 291
server accepts handled requests
 16630948 16630946 31070465
Reading: 6 Writing: 179 Waiting: 106
`))
			require.NoError(t, err)
			return
		}
		rw.WriteHeader(404)
	}))
}
