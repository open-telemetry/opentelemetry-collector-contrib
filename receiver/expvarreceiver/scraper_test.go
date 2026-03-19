// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expvarreceiver

import (
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
	allMetricsEnabled = metadata.MetricsConfig{
		ProcessRuntimeMemstatsBuckHashSys:   metadata.ProcessRuntimeMemstatsBuckHashSysMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsFrees:         metadata.ProcessRuntimeMemstatsFreesMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsGcCPUFraction: metadata.ProcessRuntimeMemstatsGcCPUFractionMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsGcSys:         metadata.ProcessRuntimeMemstatsGcSysMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsHeapAlloc:     metadata.ProcessRuntimeMemstatsHeapAllocMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsHeapIdle:      metadata.ProcessRuntimeMemstatsHeapIdleMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsHeapInuse:     metadata.ProcessRuntimeMemstatsHeapInuseMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsHeapObjects:   metadata.ProcessRuntimeMemstatsHeapObjectsMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsHeapReleased:  metadata.ProcessRuntimeMemstatsHeapReleasedMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsHeapSys:       metadata.ProcessRuntimeMemstatsHeapSysMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsLastPause:     metadata.ProcessRuntimeMemstatsLastPauseMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsLookups:       metadata.ProcessRuntimeMemstatsLookupsMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsMallocs:       metadata.ProcessRuntimeMemstatsMallocsMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsMcacheInuse:   metadata.ProcessRuntimeMemstatsMcacheInuseMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsMcacheSys:     metadata.ProcessRuntimeMemstatsMcacheSysMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsMspanInuse:    metadata.ProcessRuntimeMemstatsMspanInuseMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsMspanSys:      metadata.ProcessRuntimeMemstatsMspanSysMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsNextGc:        metadata.ProcessRuntimeMemstatsNextGcMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsNumForcedGc:   metadata.ProcessRuntimeMemstatsNumForcedGcMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsNumGc:         metadata.ProcessRuntimeMemstatsNumGcMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsOtherSys:      metadata.ProcessRuntimeMemstatsOtherSysMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsPauseTotal:    metadata.ProcessRuntimeMemstatsPauseTotalMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsStackInuse:    metadata.ProcessRuntimeMemstatsStackInuseMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsStackSys:      metadata.ProcessRuntimeMemstatsStackSysMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsSys:           metadata.ProcessRuntimeMemstatsSysMetricConfig{Enabled: true},
		ProcessRuntimeMemstatsTotalAlloc:    metadata.ProcessRuntimeMemstatsTotalAllocMetricConfig{Enabled: true},
	}
	allMetricsDisabled = metadata.MetricsConfig{
		ProcessRuntimeMemstatsBuckHashSys:   metadata.ProcessRuntimeMemstatsBuckHashSysMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsFrees:         metadata.ProcessRuntimeMemstatsFreesMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsGcCPUFraction: metadata.ProcessRuntimeMemstatsGcCPUFractionMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsGcSys:         metadata.ProcessRuntimeMemstatsGcSysMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsHeapAlloc:     metadata.ProcessRuntimeMemstatsHeapAllocMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsHeapIdle:      metadata.ProcessRuntimeMemstatsHeapIdleMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsHeapInuse:     metadata.ProcessRuntimeMemstatsHeapInuseMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsHeapObjects:   metadata.ProcessRuntimeMemstatsHeapObjectsMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsHeapReleased:  metadata.ProcessRuntimeMemstatsHeapReleasedMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsHeapSys:       metadata.ProcessRuntimeMemstatsHeapSysMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsLastPause:     metadata.ProcessRuntimeMemstatsLastPauseMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsLookups:       metadata.ProcessRuntimeMemstatsLookupsMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsMallocs:       metadata.ProcessRuntimeMemstatsMallocsMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsMcacheInuse:   metadata.ProcessRuntimeMemstatsMcacheInuseMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsMcacheSys:     metadata.ProcessRuntimeMemstatsMcacheSysMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsMspanInuse:    metadata.ProcessRuntimeMemstatsMspanInuseMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsMspanSys:      metadata.ProcessRuntimeMemstatsMspanSysMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsNextGc:        metadata.ProcessRuntimeMemstatsNextGcMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsNumForcedGc:   metadata.ProcessRuntimeMemstatsNumForcedGcMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsNumGc:         metadata.ProcessRuntimeMemstatsNumGcMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsOtherSys:      metadata.ProcessRuntimeMemstatsOtherSysMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsPauseTotal:    metadata.ProcessRuntimeMemstatsPauseTotalMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsStackInuse:    metadata.ProcessRuntimeMemstatsStackInuseMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsStackSys:      metadata.ProcessRuntimeMemstatsStackSysMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsSys:           metadata.ProcessRuntimeMemstatsSysMetricConfig{Enabled: false},
		ProcessRuntimeMemstatsTotalAlloc:    metadata.ProcessRuntimeMemstatsTotalAllocMetricConfig{Enabled: false},
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
	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(t.Context())
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
	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	expectedMetrics := pmetric.NewMetrics() // empty
	actualMetrics, err := scraper.scrape(t.Context())
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics))
}

func TestNotFoundResponse(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "expvar_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + "/nonexistent/path"
	scraper := newExpVarScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	_, err = scraper.scrape(t.Context())
	require.EqualError(t, err, "expected 200 but received 404 status code")
}

func TestBadTypeInReturnedData(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "bad_data_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + defaultPath
	scraper := newExpVarScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	_, err = scraper.scrape(t.Context())
	require.EqualError(t, err, "could not decode response body to JSON: json: cannot unmarshal string into Go struct field MemStats.memstats.Alloc of type uint64")
}

func TestJSONParseError(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "bad_data_response.txt"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + defaultPath
	scraper := newExpVarScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	_, err = scraper.scrape(t.Context())
	require.Error(t, err)
}

func TestEmptyResponseBodyError(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "bad_data_empty_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + defaultPath
	cfg.Metrics = allMetricsDisabled
	scraper := newExpVarScraper(cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	expectedMetrics := pmetric.NewMetrics() // empty
	actualMetrics, err := scraper.scrape(t.Context())
	require.EqualError(t, err, "could not decode response body to JSON: EOF")
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics))
}
