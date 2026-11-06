// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver/internal/metadata"
)

func newTestScraper(servers ...Server) *redfishScraper {
	cfg := &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		Servers:              servers,
	}
	return newScraper(cfg, receivertest.NewNopSettings(metadata.Type))
}

var scraperTestResources = []Resource{ComputerSystemResource}

func TestScraperStart(t *testing.T) {
	testCases := []struct {
		desc        string
		server      Server
		expectError bool
	}{
		{
			desc:   "No TLS",
			server: Server{BaseURL: "https://localhost:3000", User: "user", Pwd: "pwd", ComputerSystemID: "1", Resources: scraperTestResources},
		},
		{
			desc: "Missing CA File",
			server: Server{
				BaseURL:          "https://localhost:3000",
				TLS:              configtls.ClientConfig{Config: configtls.Config{CAFile: filepath.Join(t.TempDir(), "missing.crt")}},
				ComputerSystemID: "1",
				Resources:        scraperTestResources,
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			s := newTestScraper(tc.server)
			err := s.start(t.Context(), nil)
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, s.clients, 1)
		})
	}
}

func TestScraperStartTLSConfig(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("internal", "redfish", "testdata", "hpe", "systems.json"))
	}))
	defer ts.Close()

	// the test server certificate is only trusted through tls::ca_pem
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw})
	s := newTestScraper(Server{
		BaseURL:          ts.URL,
		TLS:              configtls.ClientConfig{Config: configtls.Config{CAPem: configopaque.String(caPEM)}},
		ComputerSystemID: "1",
		Resources:        scraperTestResources,
	})
	require.NoError(t, s.start(t.Context(), nil))
	require.Len(t, s.clients, 1)

	system, err := s.clients[0].GetComputerSystem()
	require.NoError(t, err)
	require.Equal(t, "otel-test-host", system.HostName)
}
