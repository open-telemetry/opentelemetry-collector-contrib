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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver/internal/metadata"
)

func TestScraper(t *testing.T) {
	nginxMock := newMockServer(t)
	defer nginxMock.Close()

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = nginxMock.URL + "/status"
	require.NoError(t, xconfmap.Validate(cfg))

	scraper := newNginxScraper(receivertest.NewNopSettings(metadata.Type), cfg)

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreMetricsOrder()))
}

func TestScraperError(t *testing.T) {
	nginxMock := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/status" {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte(`Bad status page`))
			return
		}
		rw.WriteHeader(http.StatusNotFound)
	}))
	t.Run("404", func(t *testing.T) {
		sc := newNginxScraper(receivertest.NewNopSettings(metadata.Type), &Config{
			ClientConfig: confighttp.ClientConfig{
				Endpoint: nginxMock.URL + "/badpath",
			},
		})
		err := sc.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
		_, err = sc.scrape(context.Background())
		require.Equal(t, errors.New("expected 200 response, got 404"), err)
	})

	t.Run("parse error", func(t *testing.T) {
		sc := newNginxScraper(receivertest.NewNopSettings(metadata.Type), &Config{
			ClientConfig: confighttp.ClientConfig{
				Endpoint: nginxMock.URL + "/status",
			},
		})
		err := sc.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
		_, err = sc.scrape(context.Background())
		require.ErrorContains(t, err, "Bad status page")
	})
	nginxMock.Close()
}

func TestScraperFailedStart(t *testing.T) {
	sc := newNginxScraper(receivertest.NewNopSettings(metadata.Type), &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "localhost:8080",
			TLS: configtls.ClientConfig{
				Config: configtls.Config{
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
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write([]byte(`Active connections: 291
server accepts handled requests
 16630948 16630946 31070465
Reading: 6 Writing: 179 Waiting: 106
`))
			assert.NoError(t, err)
			return
		}
		rw.WriteHeader(http.StatusNotFound)
	}))
}
