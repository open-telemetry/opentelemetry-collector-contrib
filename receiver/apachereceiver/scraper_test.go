// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachereceiver

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver/internal/metadata"
)

func TestScraper(t *testing.T) {
	tests := []struct {
		name         string
		enableNew    bool
		disableOld   bool
		expectedFile string
	}{
		{
			// Default behavior: only the original metric/attribute names are emitted.
			name:         "default_old_format",
			expectedFile: "metrics.assert.yaml",
		},
		{
			// receiver.apache.enableNewFormatMetrics only: both formats are emitted.
			name:         "both_formats",
			enableNew:    true,
			expectedFile: "metrics_both.assert.yaml",
		},
		{
			// Both gates: only the new format is emitted.
			name:         "new_format_only",
			enableNew:    true,
			disableOld:   true,
			expectedFile: "metrics_new.assert.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.enableNew {
				defer testutil.SetFeatureGateForTest(t, metadata.ReceiverApacheEnableNewFormatMetricsFeatureGate, true)()
			}
			if tt.disableOld {
				defer testutil.SetFeatureGateForTest(t, metadata.ReceiverApacheDisableOldFormatMetricsFeatureGate, true)()
			}

			apacheMock := newMockServer(t)
			defer func() { apacheMock.Close() }()

			cfg := createDefaultConfig().(*Config)
			cfg.ClientConfig.Endpoint = fmt.Sprintf("%s%s", apacheMock.URL, "/server-status?auto")
			require.NoError(t, confmap.Validate(cfg))

			serverName, port, err := parseResourceAttributes(cfg.ClientConfig.Endpoint)
			require.NoError(t, err)
			scraper := newApacheScraper(receivertest.NewNopSettings(metadata.Type), cfg, serverName, port)

			err = scraper.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)

			actualMetrics, err := scraper.scrape(t.Context())
			require.NoError(t, err)

			expectedFile := filepath.Join("testdata", "scraper", tt.expectedFile)
			// To regenerate: uncomment, run the test once, re-comment.
			// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedFile, actualMetrics))
			require.NoError(t, pmetricassert.AssertMetrics(expectedFile, actualMetrics))
		})
	}
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
	err := sc.start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestParseScoreboard(t *testing.T) {
	t.Run("test freq count", func(t *testing.T) {
		scoreboard := `S_DD_L_GGG_____W__IIII_C________________W__________________________________.........................____WR______W____W________________________C______________________________________W_W____W______________R_________R________C_________WK_W________K_____W__C__________W___R______.............................................................................................................................`
		results := parseScoreboard(scoreboard)

		require.Equal(t, int64(150), results["open"])
		require.Equal(t, int64(217), results["waiting"])
		require.Equal(t, int64(1), results["starting"])
		require.Equal(t, int64(4), results["reading"])
		require.Equal(t, int64(12), results["sending"])
		require.Equal(t, int64(2), results["keepalive"])
		require.Equal(t, int64(2), results["dnslookup"])
		require.Equal(t, int64(4), results["closing"])
		require.Equal(t, int64(1), results["logging"])
		require.Equal(t, int64(3), results["finishing"])
		require.Equal(t, int64(4), results["idle_cleanup"])
	})

	t.Run("test unknown", func(t *testing.T) {
		scoreboard := `qwertyuiopasdfghjklzxcvbnm`
		results := parseScoreboard(scoreboard)

		require.Equal(t, int64(0), results["open"])
		require.Equal(t, int64(0), results["waiting"])
		require.Equal(t, int64(0), results["starting"])
		require.Equal(t, int64(0), results["reading"])
		require.Equal(t, int64(0), results["sending"])
		require.Equal(t, int64(0), results["keepalive"])
		require.Equal(t, int64(0), results["dnslookup"])
		require.Equal(t, int64(0), results["closing"])
		require.Equal(t, int64(0), results["logging"])
		require.Equal(t, int64(0), results["finishing"])
		require.Equal(t, int64(0), results["idle_cleanup"])
		require.Equal(t, int64(26), results["unknown"])
	})

	t.Run("test empty defaults", func(t *testing.T) {
		emptyString := ""
		results := parseScoreboard(emptyString)

		require.Equal(t, int64(0), results["open"])
		require.Equal(t, int64(0), results["waiting"])
		require.Equal(t, int64(0), results["starting"])
		require.Equal(t, int64(0), results["reading"])
		require.Equal(t, int64(0), results["sending"])
		require.Equal(t, int64(0), results["keepalive"])
		require.Equal(t, int64(0), results["dnslookup"])
		require.Equal(t, int64(0), results["closing"])
		require.Equal(t, int64(0), results["logging"])
		require.Equal(t, int64(0), results["finishing"])
		require.Equal(t, int64(0), results["idle_cleanup"])
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
ConnsAsyncWriting: 2
ConnsAsyncKeepAlive: 1
ConnsAsyncClosing: 1
		`
		want := map[string]string{
			"ReqPerSec":           "719.771",
			"IdleWorkers":         "227",
			"ConnsTotal":          "110",
			"BytesPerSec":         "73.12",
			"ConnsAsyncWriting":   "2",
			"ConnsAsyncKeepAlive": "1",
			"ConnsAsyncClosing":   "1",
		}
		require.Equal(t, want, parseStats(got))
	})
}

func TestScraperError(t *testing.T) {
	t.Run("no client", func(t *testing.T) {
		sc := newApacheScraper(receivertest.NewNopSettings(metadata.Type), &Config{}, "", "")
		sc.httpClient = nil

		_, err := sc.scrape(t.Context())
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
ReqPerSec: 719.771
BytesPerSec: 73.12
Load1: 0.9
Load5: 0.4
Load15: 0.3
ConnsAsyncWriting: 2
ConnsAsyncKeepAlive: 1
ConnsAsyncClosing: 1
Total Duration: 1501
Scoreboard: S_DD_L_GGG_____W__IIII_C________________W__________________________________.........................____WR______W____W________________________C______________________________________W_W____W______________R_________R________C_________WK_W________K_____W__C__________W___R______.............................................................................................................................
`))
			assert.NoError(t, err)
			return
		}
		rw.WriteHeader(http.StatusNotFound)
	}))
}
