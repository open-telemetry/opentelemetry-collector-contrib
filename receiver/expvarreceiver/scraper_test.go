// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expvarreceiver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver/internal/metadata"
)

var (
	metricEnabled     = metadata.MetricConfig{Enabled: true}
	metricDisabled    = metadata.MetricConfig{Enabled: false}
	allMetricsEnabled = metadata.MetricsConfig{
		ProcessRuntimeMemstatsBuckHashSys:   metricEnabled,
		ProcessRuntimeMemstatsFrees:         metricEnabled,
		ProcessRuntimeMemstatsGcCPUFraction: metricEnabled,
		ProcessRuntimeMemstatsGcSys:         metricEnabled,
		ProcessRuntimeMemstatsHeapAlloc:     metricEnabled,
		ProcessRuntimeMemstatsHeapIdle:      metricEnabled,
		ProcessRuntimeMemstatsHeapInuse:     metricEnabled,
		ProcessRuntimeMemstatsHeapObjects:   metricEnabled,
		ProcessRuntimeMemstatsHeapReleased:  metricEnabled,
		ProcessRuntimeMemstatsHeapSys:       metricEnabled,
		ProcessRuntimeMemstatsLastPause:     metricEnabled,
		ProcessRuntimeMemstatsLookups:       metricEnabled,
		ProcessRuntimeMemstatsMallocs:       metricEnabled,
		ProcessRuntimeMemstatsMcacheInuse:   metricEnabled,
		ProcessRuntimeMemstatsMcacheSys:     metricEnabled,
		ProcessRuntimeMemstatsMspanInuse:    metricEnabled,
		ProcessRuntimeMemstatsMspanSys:      metricEnabled,
		ProcessRuntimeMemstatsNextGc:        metricEnabled,
		ProcessRuntimeMemstatsNumForcedGc:   metricEnabled,
		ProcessRuntimeMemstatsNumGc:         metricEnabled,
		ProcessRuntimeMemstatsOtherSys:      metricEnabled,
		ProcessRuntimeMemstatsPauseTotal:    metricEnabled,
		ProcessRuntimeMemstatsStackInuse:    metricEnabled,
		ProcessRuntimeMemstatsStackSys:      metricEnabled,
		ProcessRuntimeMemstatsSys:           metricEnabled,
		ProcessRuntimeMemstatsTotalAlloc:    metricEnabled,
	}
	allMetricsDisabled = metadata.MetricsConfig{
		ProcessRuntimeMemstatsBuckHashSys:   metricDisabled,
		ProcessRuntimeMemstatsFrees:         metricDisabled,
		ProcessRuntimeMemstatsGcCPUFraction: metricDisabled,
		ProcessRuntimeMemstatsGcSys:         metricDisabled,
		ProcessRuntimeMemstatsHeapAlloc:     metricDisabled,
		ProcessRuntimeMemstatsHeapIdle:      metricDisabled,
		ProcessRuntimeMemstatsHeapInuse:     metricDisabled,
		ProcessRuntimeMemstatsHeapObjects:   metricDisabled,
		ProcessRuntimeMemstatsHeapReleased:  metricDisabled,
		ProcessRuntimeMemstatsHeapSys:       metricDisabled,
		ProcessRuntimeMemstatsLastPause:     metricDisabled,
		ProcessRuntimeMemstatsLookups:       metricDisabled,
		ProcessRuntimeMemstatsMallocs:       metricDisabled,
		ProcessRuntimeMemstatsMcacheInuse:   metricDisabled,
		ProcessRuntimeMemstatsMcacheSys:     metricDisabled,
		ProcessRuntimeMemstatsMspanInuse:    metricDisabled,
		ProcessRuntimeMemstatsMspanSys:      metricDisabled,
		ProcessRuntimeMemstatsNextGc:        metricDisabled,
		ProcessRuntimeMemstatsNumForcedGc:   metricDisabled,
		ProcessRuntimeMemstatsNumGc:         metricDisabled,
		ProcessRuntimeMemstatsOtherSys:      metricDisabled,
		ProcessRuntimeMemstatsPauseTotal:    metricDisabled,
		ProcessRuntimeMemstatsStackInuse:    metricDisabled,
		ProcessRuntimeMemstatsStackSys:      metricDisabled,
		ProcessRuntimeMemstatsSys:           metricDisabled,
		ProcessRuntimeMemstatsTotalAlloc:    metricDisabled,
	}
)

func newMockServer(tb testing.TB, responseBodyFile string) *httptest.Server {
	fileContents, err := os.ReadFile(filepath.Clean(responseBodyFile))
	require.NoError(tb, err)
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == defaultPath {
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write(fileContents)
			assert.NoError(tb, err)
			return
		}
		rw.WriteHeader(http.StatusNotFound)
	}))
}

func TestAllMetrics(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "expvar_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + defaultPath
	cfg.Metrics = allMetricsEnabled

	scraper := newExpVarScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "metrics", "expected_all_metrics.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
		pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestNoMetrics(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "expvar_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + defaultPath
	cfg.Metrics = allMetricsDisabled
	scraper := newExpVarScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	expectedMetrics := pmetric.NewMetrics() // empty
	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics))
}

func TestNotFoundResponse(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "expvar_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + "/nonexistent/path"
	scraper := newExpVarScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	_, err = scraper.scrape(context.Background())
	require.EqualError(t, err, "expected 200 but received 404 status code")
}

func TestBadTypeInReturnedData(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "bad_data_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + defaultPath
	scraper := newExpVarScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	_, err = scraper.scrape(context.Background())
	require.EqualError(t, err, "could not decode response body to JSON: json: cannot unmarshal string into Go struct field MemStats.memstats.Alloc of type uint64")
}

func TestJSONParseError(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "bad_data_response.txt"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + defaultPath
	scraper := newExpVarScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	_, err = scraper.scrape(context.Background())
	require.Error(t, err)
}

func TestEmptyResponseBodyError(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "bad_data_empty_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + defaultPath
	cfg.Metrics = allMetricsDisabled
	scraper := newExpVarScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	expectedMetrics := pmetric.NewMetrics() // empty
	actualMetrics, err := scraper.scrape(context.Background())
	require.EqualError(t, err, "could not decode response body to JSON: EOF")
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics))
}
