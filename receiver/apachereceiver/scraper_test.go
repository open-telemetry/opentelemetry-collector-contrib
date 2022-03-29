// Copyright  OpenTelemetry Authors
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

package apachereceiver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

func TestScraper(t *testing.T) {
	apacheMock := newMockServer(t)
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = fmt.Sprintf("%s%s", apacheMock.URL, "/server-status?auto")
	require.NoError(t, cfg.Validate())

	scraper := newApacheScraper(componenttest.NewNopTelemetrySettings(), cfg)

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
}

func TestScraperFailedStart(t *testing.T) {
	sc := newApacheScraper(componenttest.NewNopTelemetrySettings(), &Config{
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

func TestParseScoreboard(t *testing.T) {

	t.Run("test freq count", func(t *testing.T) {
		scoreboard := `S_DD_L_GGG_____W__IIII_C________________W__________________________________.........................____WR______W____W________________________C______________________________________W_W____W______________R_________R________C_________WK_W________K_____W__C__________W___R______.............................................................................................................................`
		results := parseScoreboard(scoreboard)

		require.EqualValues(t, int64(150), results["open"])
		require.EqualValues(t, int64(217), results["waiting"])
		require.EqualValues(t, int64(1), results["starting"])
		require.EqualValues(t, int64(4), results["reading"])
		require.EqualValues(t, int64(12), results["sending"])
		require.EqualValues(t, int64(2), results["keepalive"])
		require.EqualValues(t, int64(2), results["dnslookup"])
		require.EqualValues(t, int64(4), results["closing"])
		require.EqualValues(t, int64(1), results["logging"])
		require.EqualValues(t, int64(3), results["finishing"])
		require.EqualValues(t, int64(4), results["idle_cleanup"])
	})

	t.Run("test unknown", func(t *testing.T) {
		scoreboard := `qwertyuiopasdfghjklzxcvbnm`
		results := parseScoreboard(scoreboard)

		require.EqualValues(t, int64(0), results["open"])
		require.EqualValues(t, int64(0), results["waiting"])
		require.EqualValues(t, int64(0), results["starting"])
		require.EqualValues(t, int64(0), results["reading"])
		require.EqualValues(t, int64(0), results["sending"])
		require.EqualValues(t, int64(0), results["keepalive"])
		require.EqualValues(t, int64(0), results["dnslookup"])
		require.EqualValues(t, int64(0), results["closing"])
		require.EqualValues(t, int64(0), results["logging"])
		require.EqualValues(t, int64(0), results["finishing"])
		require.EqualValues(t, int64(0), results["idle_cleanup"])
		require.EqualValues(t, int64(26), results["unknown"])
	})

	t.Run("test empty defaults", func(t *testing.T) {
		emptyString := ""
		results := parseScoreboard(emptyString)

		require.EqualValues(t, int64(0), results["open"])
		require.EqualValues(t, int64(0), results["waiting"])
		require.EqualValues(t, int64(0), results["starting"])
		require.EqualValues(t, int64(0), results["reading"])
		require.EqualValues(t, int64(0), results["sending"])
		require.EqualValues(t, int64(0), results["keepalive"])
		require.EqualValues(t, int64(0), results["dnslookup"])
		require.EqualValues(t, int64(0), results["closing"])
		require.EqualValues(t, int64(0), results["logging"])
		require.EqualValues(t, int64(0), results["finishing"])
		require.EqualValues(t, int64(0), results["idle_cleanup"])
	})
}

func TestParseStats(t *testing.T) {
	t.Run("with empty value", func(t *testing.T) {
		emptyString := ""
		require.EqualValues(t, map[string]string{}, parseStats(emptyString))
	})
	t.Run("with multi colons", func(t *testing.T) {
		got := "CurrentTime: Thursday, 17-Jun-2021 14:06:32 UTC"
		want := map[string]string{
			"CurrentTime": "Thursday, 17-Jun-2021 14:06:32 UTC",
		}
		require.EqualValues(t, want, parseStats(got))
	})
	t.Run("with header/footer", func(t *testing.T) {
		got := `localhost
ReqPerSec: 719.771
IdleWorkers: 227
ConnsTotal: 110
		`
		want := map[string]string{
			"ReqPerSec":   "719.771",
			"IdleWorkers": "227",
			"ConnsTotal":  "110",
		}
		require.EqualValues(t, want, parseStats(got))
	})
}

func TestScraperError(t *testing.T) {
	t.Run("no client", func(t *testing.T) {
		sc := newApacheScraper(componenttest.NewNopTelemetrySettings(), &Config{})
		sc.httpClient = nil

		_, err := sc.scrape(context.Background())
		require.Error(t, err)
		require.EqualValues(t, errors.New("failed to connect to Apache HTTPd"), err)
	})
}

func newMockServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.String() == "/server-status?auto" {
			rw.WriteHeader(200)
			_, err := rw.Write([]byte(`ServerUptimeSeconds: 410
Total Accesses: 14169
Total kBytes: 20910
BusyWorkers: 13
IdleWorkers: 227
ConnsTotal: 110
Scoreboard: S_DD_L_GGG_____W__IIII_C________________W__________________________________.........................____WR______W____W________________________C______________________________________W_W____W______________R_________R________C_________WK_W________K_____W__C__________W___R______.............................................................................................................................
`))
			require.NoError(t, err)
			return
		}
		rw.WriteHeader(404)
	}))
}
