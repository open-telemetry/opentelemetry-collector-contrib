// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

// handler function for mock server
func mockIndexerThroughput(w http.ResponseWriter, _ *http.Request) {
	status := http.StatusOK
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"links":{},"origin":"https://somehost:8089/services/server/introspection/indexer","updated":"2023-07-31T21:41:07+00:00","generator":{"build":"82c987350fde","version":"9.0.1"},"entry":[{"name":"indexer","id":"https://34.213.134.166:8089/services/server/introspection/indexer/indexer","updated":"1970-01-01T00:00:00+00:00","links":{"alternate":"/services/server/introspection/indexer/indexer","list":"/services/server/introspection/indexer/indexer","edit":"/services/server/introspection/indexer/indexer"},"author":"system","acl":{"app":"","can_list":true,"can_write":true,"modifiable":false,"owner":"system","perms":{"read":["admin","splunk-system-role"],"write":["admin","splunk-system-role"]},"removable":false,"sharing":"system"},"content":{"average_KBps":25.579690815904478,"eai:acl":null,"reason":"","status":"normal"}}],"paging":{"total":1,"perPage":30,"offset":0},"messages":[]}`))
}

// mock server create
func createMockServer() *httptest.Server {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case "/services/server/introspection/indexer":
			mockIndexerThroughput(w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))

	return ts
}

func TestScraper(t *testing.T) {
	ts := createMockServer()
	defer ts.Close()

	// in the future add more metrics
	metricsettings := metadata.MetricsBuilderConfig{}
	metricsettings.Metrics.SplunkServerIntrospectionIndexerThroughput.Enabled = true

	cfg := &Config{
		Username:          "admin",
		Password:          "securityFirst",
		MaxSearchWaitTime: 11 * time.Second,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: ts.URL,
		},
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
			InitialDelay:       1 * time.Second,
		},
		MetricsBuilderConfig: metricsettings,
	}

	require.NoError(t, cfg.Validate())

	scraper := newSplunkMetricsScraper(receivertest.NewNopCreateSettings(), cfg)
    client := newSplunkEntClient(cfg)
    scraper.splunkClient = &client

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")

	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}
