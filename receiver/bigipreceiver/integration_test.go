// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package bigipreceiver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestIntegration(t *testing.T) {
	mockServer := setupMockIControlServer(t)
	defer mockServer.Close()

	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(t *testing.T, cfg component.Config, ci *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = 100 * time.Millisecond
				rCfg.Endpoint = mockServer.URL
				// Any username/password will work with mocked server right now
				rCfg.Username = "otelu"
				rCfg.Password = "otelp"
			}),
		scraperinttest.WithCompareOptions(
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp()),
	).Run(t)
}

const (
	authHeader = "X-F5-Auth-Token"

	loginURISuffix                  = "/authn/login"
	getVirtualServersURISuffix      = "/ltm/virtual"
	getVirtualServersStatsURISuffix = "/ltm/virtual/stats"
	getPoolsStatsURISuffix          = "/ltm/pool/stats"
	getPoolMembersStatsURISuffix    = "/members/stats"
	getNodesStatsURISuffix          = "/ltm/node/stats"

	mockLoginResponseFile               = "login_response.json"
	mockVirtualServersResponseFile      = "virtual_servers_response.json"
	mockVirtualServersStatsResponseFile = "virtual_servers_stats_response.json"
	mockPoolsStatsResponseFile          = "pools_stats_response.json"
	mockNodesStatsResponseFile          = "nodes_stats_response.json"
	poolMembersStatsResponseFileSuffix  = "_pool_members_stats_response.json"
)

func setupMockIControlServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Setup responses before creating the server
	mockLoginResponse := createMockServerResponseData(t, mockLoginResponseFile)
	mockVirtualServersResponse := createMockServerResponseData(t, mockVirtualServersResponseFile)
	mockVirtualServersStatsResponse := createMockServerResponseData(t, mockVirtualServersStatsResponseFile)
	mockPoolsStatsResponse := createMockServerResponseData(t, mockPoolsStatsResponseFile)
	mockNodesStatsResponse := createMockServerResponseData(t, mockNodesStatsResponseFile)

	type loginBody struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// Setup mock iControl REST API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock login responses
		var err error
		if strings.HasSuffix(r.RequestURI, loginURISuffix) {
			var body loginBody
			err = json.NewDecoder(r.Body).Decode(&body)
			require.NoError(t, err)
			if body.Username == "" || body.Password == "" || r.Method != "POST" {
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				_, err = w.Write(mockLoginResponse)
				require.NoError(t, err)
			}

			return
		}

		// Mock response if no auth token header
		if r.Header.Get(authHeader) == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case strings.HasSuffix(r.RequestURI, getVirtualServersURISuffix):
			_, err = w.Write(mockVirtualServersResponse)
		case strings.HasSuffix(r.RequestURI, getVirtualServersStatsURISuffix):
			_, err = w.Write(mockVirtualServersStatsResponse)
		case strings.HasSuffix(r.RequestURI, getPoolsStatsURISuffix):
			_, err = w.Write(mockPoolsStatsResponse)
		case strings.HasSuffix(r.RequestURI, getNodesStatsURISuffix):
			_, err = w.Write(mockNodesStatsResponse)
		case strings.HasSuffix(r.RequestURI, getPoolMembersStatsURISuffix):
			// Assume pool member response files follow a specific file pattern based of pool name
			poolURI := strings.TrimSuffix(r.RequestURI, getPoolMembersStatsURISuffix)
			poolURIParts := strings.Split(poolURI, "/")
			poolName := strings.ReplaceAll(poolURIParts[len(poolURIParts)-1], "~", "_")
			poolMembersStatsData := createMockServerResponseData(t, poolName+poolMembersStatsResponseFileSuffix)
			_, err = w.Write(poolMembersStatsData)
		default:
			w.WriteHeader(http.StatusBadRequest)
			err = nil
		}
		require.NoError(t, err)
	}))

	return server
}

func createMockServerResponseData(t *testing.T, fileName string) []byte {
	t.Helper()
	fullPath := filepath.Join("testdata", "integration", "mock_server", fileName)

	data, err := os.ReadFile(fullPath)
	require.NoError(t, err)

	return data
}
