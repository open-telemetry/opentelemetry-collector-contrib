// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mesosreceiver

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestScraper(t *testing.T) {
	mesosMock := newMockServer(t)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = fmt.Sprintf("%s%s", mesosMock.URL, "/metrics/snapshot")
	require.NoError(t, component.ValidateConfig(cfg))
	//fmt.Println(cfg.Endpoint)
	serverName, port, err := parseResourseAttributes(cfg.Endpoint)
	require.NoError(t, err)
	scraper := newMesosScraper(receivertest.NewNopCreateSettings(), cfg, serverName, port)

	err = scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)
	resp, _ := scraper.GetStats()
	fmt.Println(parseStats(resp))

	expectedFile := filepath.Join("testdata", "scraper", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScraperFailedStart(t *testing.T) {
	sc := newMesosScraper(receivertest.NewNopCreateSettings(), &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "remote-docker-ageorgiades:5050",
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting: configtls.TLSSetting{
					CAFile: "/non/existent",
				},
			},
		},
	},
		"remote-docker-ageorgiades",
		"5050")
	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestConversionFunction(t *testing.T) {
	// in the scraper total memory metric goes from string mb into float64 Bytes into string bytes
	totalMemory := "15015"
	actualValue := convMegabytesToBytes(totalMemory)
	expectedValue := "15015000000"
	require.EqualValues(t, actualValue, expectedValue)

}

func newMockServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.String() == "/metrics/snapshot" {
			rw.WriteHeader(200)
			_, err := rw.Write([]byte(`{"master/gpus_percent":0.0,
			"master/cpus_percent":0.0,
			"master/mem_percent":0.0,
			"master/mem_total":15015.0,
			"master/uptime_secs":445202.377993984,
			"system/load_15min":0.0,
			"master/slaves_active":1.0,
			"master/slaves_connected":1.0,
			"master/slaves_disconnected":0.0,
			"master/slaves_inactive":0.0,
			"master/tasks_failed":0.0,
			"master/tasks_finished":0.0}
`))
			require.NoError(t, err)
			return
		}
		rw.WriteHeader(404)
	}))
}
