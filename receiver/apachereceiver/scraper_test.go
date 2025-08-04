// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachereceiver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver/internal/metadata"
)

func TestScraper(t *testing.T) {
	apacheMock := newMockServer(t)
	defer func() { apacheMock.Close() }()

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = fmt.Sprintf("%s%s", apacheMock.URL, "/server-status?auto")
	require.NoError(t, xconfmap.Validate(cfg))

	serverName, port, err := parseResourceAttributes(cfg.Endpoint)
	require.NoError(t, err)
	scraper := newApacheScraper(receivertest.NewNopSettings(metadata.Type), cfg, serverName, port)

	err = scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	actualMetrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	url, err := url.Parse(apacheMock.URL)
	require.NoError(t, err)

	expectedMetrics.ResourceMetrics().At(0).Resource().Attributes().PutStr("apache.server.port", url.Port())

	// The port is random, so we shouldn't check if this value matches.
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
		pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
}

func TestScraperFailedStart(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "localhost:8080"
	clientConfig.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "/non/existent",
		},
	}
	sc := newApacheScraper(receivertest.NewNopSettings(metadata.Type), &Config{
		ClientConfig: clientConfig,
	},
		"localhost",
		"8080")
	err := sc.start(context.Background(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestParseScoreboard(t *testing.T) {
	t.Run("test freq count", func(t *testing.T) {
		scoreboard := `S_DD_L_GGG_____W__IIII_C________________W__________________________________.........................____WR______W____W________________________C______________________________________W_W____W______________R_________R________C_________WK_W________K_____W__C__________W___R______.............................................................................................................................`
		results := parseScoreboard(scoreboard)

		require.Equal(t, int64(150), results[metadata.AttributeScoreboardStateOpen])
		require.Equal(t, int64(217), results[metadata.AttributeScoreboardStateWaiting])
		require.Equal(t, int64(1), results[metadata.AttributeScoreboardStateStarting])
		require.Equal(t, int64(4), results[metadata.AttributeScoreboardStateReading])
		require.Equal(t, int64(12), results[metadata.AttributeScoreboardStateSending])
		require.Equal(t, int64(2), results[metadata.AttributeScoreboardStateKeepalive])
		require.Equal(t, int64(2), results[metadata.AttributeScoreboardStateDnslookup])
		require.Equal(t, int64(4), results[metadata.AttributeScoreboardStateClosing])
		require.Equal(t, int64(1), results[metadata.AttributeScoreboardStateLogging])
		require.Equal(t, int64(3), results[metadata.AttributeScoreboardStateFinishing])
		require.Equal(t, int64(4), results[metadata.AttributeScoreboardStateIdleCleanup])
	})

	t.Run("test unknown", func(t *testing.T) {
		scoreboard := `qwertyuiopasdfghjklzxcvbnm`
		results := parseScoreboard(scoreboard)

		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateOpen])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateWaiting])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateStarting])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateReading])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateSending])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateKeepalive])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateDnslookup])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateClosing])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateLogging])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateFinishing])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateIdleCleanup])
		require.Equal(t, int64(26), results[metadata.AttributeScoreboardStateUnknown])
	})

	t.Run("test empty defaults", func(t *testing.T) {
		emptyString := ""
		results := parseScoreboard(emptyString)

		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateOpen])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateWaiting])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateStarting])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateReading])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateSending])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateKeepalive])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateDnslookup])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateClosing])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateLogging])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateFinishing])
		require.Equal(t, int64(0), results[metadata.AttributeScoreboardStateIdleCleanup])
	})
}

func TestParseStats(t *testing.T) {
	t.Run("with empty value", func(t *testing.T) {
		emptyString := ""
		require.Equal(t, map[string]string{}, parseStats(emptyString))
	})
	t.Run("with multi colons", func(t *testing.T) {
		got := "CurrentTime: Thursday, 17-Jun-2021 14:06:32 UTC"
		want := map[string]string{
			"CurrentTime": "Thursday, 17-Jun-2021 14:06:32 UTC",
		}
		require.Equal(t, want, parseStats(got))
	})
	t.Run("with header/footer", func(t *testing.T) {
		got := `localhost
ReqPerSec: 719.771
IdleWorkers: 227
ConnsTotal: 110
BytesPerSec: 73.12
		`
		want := map[string]string{
			"ReqPerSec":   "719.771",
			"IdleWorkers": "227",
			"ConnsTotal":  "110",
			"BytesPerSec": "73.12",
		}
		require.Equal(t, want, parseStats(got))
	})
}

func TestScraperError(t *testing.T) {
	t.Run("no client", func(t *testing.T) {
		sc := newApacheScraper(receivertest.NewNopSettings(metadata.Type), &Config{}, "", "")
		sc.httpClient = nil

		_, err := sc.scrape(context.Background())
		require.Error(t, err)
		require.Equal(t, errors.New("failed to connect to Apache HTTPd"), err)
	})
}

func newMockServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.String() == "/server-status?auto" {
			rw.WriteHeader(http.StatusOK)
			_, err := rw.Write([]byte(`ServerUptimeSeconds: 410
Total Accesses: 14169
Total kBytes: 20910
BusyWorkers: 13
IdleWorkers: 227
ConnsTotal: 110
CPUChildrenSystem: 0.01
CPUChildrenUser: 0.02
CPUSystem: 0.03
CPUUser: 0.04
CPULoad: 0.66
Load1: 0.9
Load5: 0.4
Load15: 0.3
Total Duration: 1501
Scoreboard: S_DD_L_GGG_____W__IIII_C________________W__________________________________.........................____WR______W____W________________________C______________________________________W_W____W______________R_________R________C_________WK_W________K_____W__C__________W___R______.............................................................................................................................
`))
			assert.NoError(t, err)
			return
		}
		rw.WriteHeader(http.StatusNotFound)
	}))
}
