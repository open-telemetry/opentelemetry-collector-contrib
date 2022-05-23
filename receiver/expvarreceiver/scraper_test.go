// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package expvarreceiver

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver/internal/metadata"
)

var (
	metricEnabled     = metadata.MetricSettings{Enabled: true}
	metricDisabled    = metadata.MetricSettings{Enabled: false}
	allMetricsEnabled = metadata.MetricsSettings{
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
	allMetricsDisabled = metadata.MetricsSettings{
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
	file, err := os.Open(responseBodyFile)
	assert.NoError(tb, err)
	fileContents, err := ioutil.ReadAll(file)
	require.NoError(tb, err)
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/debug/vars" {
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write(fileContents)
			require.NoError(tb, err)
			return
		}
		rw.WriteHeader(http.StatusNotFound)
	}))
}

func TestAllMetrics(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "expvar_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + "/debug/vars"
	cfg.MetricsConfig = allMetricsEnabled

	scraper := newExpVarScraper(cfg, componenttest.NewNopReceiverCreateSettings())
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "metrics", "expected_all_metrics.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
}

func TestNoMetrics(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "expvar_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + "/debug/vars"
	cfg.MetricsConfig = allMetricsDisabled
	scraper := newExpVarScraper(cfg, componenttest.NewNopReceiverCreateSettings())
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	expectedMetrics := pmetric.NewMetrics() // empty
	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)
	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
}

func TestNotFoundResponse(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "expvar_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + "/nonexistent/path"
	scraper := newExpVarScraper(cfg, componenttest.NewNopReceiverCreateSettings())
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	_, err = scraper.scrape(context.Background())
	require.EqualError(t, err, "expected 200 but received 404 status code")
}

func TestBadTypeInReturnedData(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "bad_data_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + "/debug/vars"
	scraper := newExpVarScraper(cfg, componenttest.NewNopReceiverCreateSettings())
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	_, err = scraper.scrape(context.Background())
	require.EqualError(t, err, "could not decode response body to JSON: json: cannot unmarshal string into Go struct field MemStats.memstats.Alloc of type uint64")
}

func TestJSONParseError(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "bad_data_response.txt"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + "/debug/vars"
	scraper := newExpVarScraper(cfg, componenttest.NewNopReceiverCreateSettings())
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	_, err = scraper.scrape(context.Background())
	require.NotNil(t, err)
}

func TestEmptyResponseBodyError(t *testing.T) {
	ms := newMockServer(t, filepath.Join("testdata", "response", "bad_data_empty_response.json"))
	defer ms.Close()
	cfg := newDefaultConfig().(*Config)
	cfg.Endpoint = ms.URL + "/debug/vars"
	cfg.MetricsConfig = allMetricsDisabled
	scraper := newExpVarScraper(cfg, componenttest.NewNopReceiverCreateSettings())
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	expectedMetrics := pmetric.NewMetrics() // empty
	actualMetrics, err := scraper.scrape(context.Background())
	require.EqualError(t, err, "could not decode response body to JSON: EOF")
	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
}
